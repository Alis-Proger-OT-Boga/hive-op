package main

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"github.com/ethereum-optimism/optimism/op-node/cmd/p2p"
	"github.com/ethereum-optimism/optimism/op-node/flags"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
	"math/big"
	"strconv"
	"sync"
	"time"

	"github.com/ethereum-optimism/optimism/op-node/eth"
	"github.com/ethereum-optimism/optimism/op-node/rollup/driver"
	"github.com/stretchr/testify/require"

	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/optimism"
)

const replicaCount = 2
const maxReplicaLag = 5

func main() {
	suite := hivesim.Suite{
		Name:        "optimism p2p",
		Description: "This suite runs the P2P tests",
	}

	// Add tests for full nodes.
	suite.Add(&hivesim.TestSpec{
		Name:        "simple p2p testnet",
		Description: `This suite runs the a testnet with P2P set up`,
		Run:         func(t *hivesim.T) { runP2PTests(t) },
	})

	sim := hivesim.New()
	hivesim.MustRunSuite(sim, suite)
}

// runP2PTests runs the P2P tests between the sequencer and verifier.
func runP2PTests(t *hivesim.T) {
	d := optimism.NewDevnet(t)

	d.InitChain(30, 4, 30, nil)
	d.RollupCfg.P2PSequencerAddress = crypto.PubkeyToAddress(d.Secrets.SequencerP2P.PublicKey)
	d.AddEth1() // l1 eth1 node is required for l2 config init
	d.WaitUpEth1(0, time.Second*10)

	var wg sync.WaitGroup
	for i := 0; i <= replicaCount; i++ {
		isSeq := i == 0
		key, err := d.MnemonicCfg.P2PKeyFor(i)
		require.NoError(t, err)
		p2pPrivFile := optimism.StringFile(optimism.DefaultP2PPrivPath, hex.EncodeToString(optimism.EncodePrivKey(key)))
		seqPrivFile := optimism.StringFile(optimism.DefaultP2PSequencerPrivPath, hex.EncodeToString(optimism.EncodePrivKey(d.Secrets.SequencerP2P)))

		d.AddOpL2()
		d.AddOpNode(0, i, isSeq, p2pPrivFile, seqPrivFile, optimism.HiveUnpackParams{
			flags.AdvertiseTCPPort.EnvVar: strconv.Itoa(9300 + i),
			flags.ListenTCPPort.EnvVar:    strconv.Itoa(9300 + i),
		}.Params())

		if isSeq {
			d.AddOpBatcher(0, 0, 0, optimism.HiveUnpackParams{}.Params())
			d.AddOpProposer(0, 0, 0)
		}

		wg.Add(1)
		go func(i int) {
			d.WaitUpOpL2Engine(i, time.Second*10)
			wg.Done()
		}(i)
	}

	t.Log("waiting for nodes to come up")
	wg.Wait()

	for i := 1; i <= replicaCount; i++ {
		node := d.GetOpNode(i)
		p2pClient := node.P2PClient()

		for j := 0; j <= replicaCount; j++ {
			if i == j {
				continue
			}

			peerAddr, err := addrFor(d, j)
			require.NoError(t, err)
			t.Logf("peering node %d (%s) with %d", j, peerAddr, i)
			require.NoError(t, p2pClient.ConnectPeer(context.Background(), peerAddr))
		}
	}

	seqEng := d.GetOpL2Engine(0)
	seqEthCl := seqEng.EthClient()
	seqRollupCl := d.GetOpNode(0).RollupClient()
	sender := d.L2Vault.CreateAccount(context.Background(), seqEthCl, big.NewInt(params.Ether))

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	readyCh := make(chan struct{})
	t.Log("awaiting initial sync")
	go func() {
		tick := time.NewTicker(250 * time.Millisecond)
		for {
			select {
			case <-tick.C:
				seqHead, err := seqRollupCl.SyncStatus(ctx)
				require.NoError(t, err)
				if seqHead.UnsafeL2.Number == seqHead.SafeL2.Number {
					continue
				}
				ready := true
				for i := 1; i <= replicaCount; i++ {
					repRollupCl := d.GetOpNode(i).RollupClient()
					repHead, err := repRollupCl.SyncStatus(ctx)
					require.NoError(t, err)
					if seqHead.UnsafeL2.Number-repHead.UnsafeL2.Number >= 2 {
						ready = false
						break
					}
				}
				if ready {
					readyCh <- struct{}{}
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	select {
	case <-readyCh:
		cancel()
		t.Log("initial sync complete")
	case <-ctx.Done():
		t.Fatalf("timed out waiting for initial sync")
	}

	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Minute)
	errCh := make(chan error, 20)
	defer cancel()

	getSyncStat := func(ctx context.Context, i int) *driver.SyncStatus {
		cl := d.GetOpNode(i).RollupClient()
		seqStat, err := cl.SyncStatus(ctx)
		require.NoError(t, err)
		t.Log(fmt.Sprintf("replica-%d", i),
			"currentL1", seqStat.CurrentL1.TerminalString(),
			"headL1", seqStat.HeadL1.TerminalString(),
			"finalizedL2", seqStat.FinalizedL2.TerminalString(),
			"safeL2", seqStat.SafeL2.TerminalString(),
			"unsafeL2", seqStat.UnsafeL2.TerminalString())
		return seqStat
	}

	checkCanon := func(i int, head uint64, id eth.BlockID) error {
		if head-id.Number > maxReplicaLag {
			return fmt.Errorf("replica %d: too far behind sequencer. seq head: %d, replica head: %d", i, head, id.Number)
		}
		bl, err := seqEthCl.BlockByNumber(ctx, big.NewInt(int64(id.Number)))
		if err != nil {
			return fmt.Errorf("replica %d: sequencer does not have block at height %d", i, id.Number)
		}
		if h := bl.Hash(); h != id.Hash {
			return fmt.Errorf("replica %d: sequencer diverged, height %d does not match: sequencer: %s <> verifier: %s", i, h, id.Hash)
		}
		return nil
	}

	go func() {
		tick := time.NewTicker(100 * time.Millisecond)
		for {
			select {
			case <-tick.C:
				nonce, err := seqEthCl.NonceAt(ctx, sender, nil)
				if err != nil {
					errCh <- err
					return
				}
				tx := types.NewTx(&types.DynamicFeeTx{
					ChainID:   optimism.L2ChainIDBig,
					Nonce:     nonce,
					Gas:       75000,
					GasTipCap: big.NewInt(1),
					GasFeeCap: big.NewInt(2),
					Value:     big.NewInt(0.0001 * params.Ether),
				})
				tx, err = d.L2Vault.SignTransaction(sender, tx)
				if err != nil {
					errCh <- err
					return
				}
				require.NoError(t, seqEthCl.SendTransaction(ctx, tx))
				_, err = optimism.WaitReceipt(ctx, seqEthCl, tx.Hash())
				if err != nil {
					errCh <- err
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	go func() {
		tick := time.NewTicker(100 * time.Millisecond)
		for {
			select {
			case <-tick.C:
				head, err := seqEthCl.BlockByNumber(ctx, nil)
				if err != nil {
					errCh <- err
					return
				}

				for i := 1; i <= replicaCount; i++ {
					seqStat := getSyncStat(ctx, i)
					if err := checkCanon(i, head.NumberU64(), seqStat.UnsafeL2.ID()); err != nil {
						errCh <- err
						return
					}
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	select {
	case <-time.NewTimer(time.Minute).C:
		break
	case err := <-errCh:
		t.Fatalf("unhandled error: %v", err)
	}

	cancel()
}

func addrFor(d *optimism.Devnet, i int) (string, error) {
	ip := d.GetOpNode(i).IP.String()
	key, err := d.MnemonicCfg.P2PKeyFor(i)
	if err != nil {
		return "", err
	}
	return asMultiAddr(ip, key, 9300+i)
}

func asMultiAddr(ip string, privKey *ecdsa.PrivateKey, port int) (string, error) {
	keyB := []byte(hex.EncodeToString(optimism.EncodePrivKey(privKey)))
	peerID, err := p2p.Priv2PeerID(bytes.NewReader(keyB))
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("/ip4/%s/tcp/%d/p2p/%s", ip, port, peerID), nil
}
