package main

import (
	"context"
	"errors"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils"
	"github.com/ethereum-optimism/optimism/op-node/rollup/derive"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/optimism"
	"github.com/stretchr/testify/require"
	"math/big"
	"time"
)

func depositWhileSequencerDownTest(t *hivesim.T, env *optimism.TestEnv) {
	d := env.Devnet
	depositCount := 3

	require.NoError(t, optimism.StartSequencerDevnet(env.Context, d, &optimism.SequencerDevnetParams{
		MaxSeqDrift:   120,
		SeqWindowSize: 2,
		ChanTimeout:   30,
	}))

	l1 := env.Devnet.L1Client(0)
	l2Sequencer := env.Devnet.L2Client(0)
	l1Vault := env.Devnet.L1Vault

	// Start a replica
	d.T.Log("starting replica")
	d.AddOpL2()
	d.WaitUpOpL2Engine(1, time.Second*10)
	d.AddOpNode(0, 1, false)

	d.T.Log("depositing funds")
	depositor := l1Vault.CreateAccount(env.TimeoutCtx(time.Minute), l1, big.NewInt(params.Ether))
	doDeposit(t, env, depositor)

	// Shut down the sequencer, batcher, and proposer
	d.T.Log("shutting down sequencer services")
	require.NoError(t, d.GetOpNode(0).Shutdown())
	require.NoError(t, d.ShutdownBatcher())
	require.NoError(t, d.ShutdownProposer())

	// Perform a bunch of additional deposits to get past the sequencer window
	d.T.Log("performing deposit post-shutdown")
	for i := 0; i < depositCount; i++ {
		doDeposit(t, env, depositor)
	}
	d.T.Logf("deposit complete")

	// Check the balance post-deposit
	d.T.Log("checking balance")
	l2Replica := env.Devnet.L2Client(1)
	balance, err := l2Replica.BalanceAt(env.TimeoutCtx(time.Minute), depositor, nil)
	require.NoError(t, err)
	require.Equal(t, big.NewInt(int64(depositCount+1)*1_000_000), balance)

	// Grab replica head block
	head, err := l2Replica.HeaderByNumber(env.TimeoutCtx(5*time.Second), nil)
	require.NoError(t, err)

	// Smoke check to make sure that the sequencer actually halted
	headSeq, err := l2Sequencer.HeaderByNumber(env.TimeoutCtx(5*time.Second), nil)
	require.NoError(t, err)
	require.NotEqual(t, head.Number, headSeq.Number)

	// Bring back the sequencer
	d.T.Log("restarting sequencer services")
	d.AddOpNode(0, 0, true)
	d.AddOpBatcher(0, 0, 2)
	d.AddOpProposer(0, 0, 2)

	// Wait for the sequencer to get back into sync with the replica
	d.T.Log("waiting for sequencer to sync")
	require.NoError(t, e2eutils.WaitFor(env.TimeoutCtx(2*time.Minute), time.Second, func() (bool, error) {
		// Have to create a separate context here because the TimeoutCtx utility
		// will cancel the parent context when the next one is called. This doesn't
		// work with nested contexts like this, since the outer context will be
		// cancelled and return an error improperly.
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, err := l2Sequencer.HeaderByHash(ctx, head.Hash())
		if err == nil {
			d.T.Log("sequencer synced")
			return true, nil
		}
		if errors.Is(err, ethereum.NotFound) {
			d.T.Log("not found")
			return false, nil
		}
		d.T.Logf("error checking sync state: %v", err)
		return false, err
	}))
}

func doDeposit(t *hivesim.T, env *optimism.TestEnv, depositor common.Address) {
	l1 := env.Devnet.L1Client(0)
	// Use the replica because the sequencer will be taken down
	l2 := env.Devnet.L2Client(1)
	depositContract := env.Devnet.Bindings.BindingsL1.OptimismPortal
	l1Vault := env.Devnet.L1Vault

	opts := l1Vault.KeyedTransactor(depositor)
	opts.Value = big.NewInt(1_000_000)
	opts.GasLimit = 3_000_000
	tx, err := depositContract.DepositTransaction(opts, depositor, common.Big0, 1_000_000, false, nil)
	require.NoError(t, err)
	receipt, err := optimism.WaitReceiptOK(env.TimeoutCtx(time.Minute), l1, tx.Hash())
	require.NoError(t, err)

	reconstructedDep, err := derive.UnmarshalDepositLogEvent(receipt.Logs[0])
	require.NoError(t, err, "could not reconstruct L2 deposit")
	tx = types.NewTx(reconstructedDep)
	_, err = optimism.WaitReceiptOK(env.TimeoutCtx(45*time.Second), l2, tx.Hash())
	require.NoError(t, err)
}
