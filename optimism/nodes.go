package optimism

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/ethereum-optimism/optimism/op-node/eth"
	"github.com/ethereum-optimism/optimism/op-node/p2p"
	"github.com/ethereum-optimism/optimism/op-proposer/rollupclient"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/hive/hivesim"
	"math/big"
	"strings"
	"sync"
	"time"
)

// These ports are exposed on the docker containers, and accessible via the docker network that the hive test runs in.
// These are container-ports: they are not exposed to the host,
// and so multiple containers can use the same port.
// Some eth1 client definitions hardcode them, others make them configurable, these should not be changed.
const (
	HttpRPCPort = 8545
	WsRPCPort   = 8546
	EnginePort  = 8551
	// RollupRPCPort is set to the default EL RPC port,
	// since Hive defaults to RPC / caching / liveness checks on this port.
	RollupRPCPort = 8545
)

type ELNode struct {
	*hivesim.Client
}

func (e *ELNode) HttpRpcEndpoint() string {
	return fmt.Sprintf("http://%v:%d", e.IP, HttpRPCPort)
}

func (e *ELNode) WsRpcEndpoint() string {
	// carried over from older mergenet ws connection problems, idk why clients are different
	switch e.Client.Type {
	case "besu":
		return fmt.Sprintf("ws://%v:%d/ws", e.IP, WsRPCPort)
	case "nethermind":
		return fmt.Sprintf("http://%v:%d/ws", e.IP, WsRPCPort) // upgrade
	default:
		return fmt.Sprintf("ws://%v:%d", e.IP, WsRPCPort)
	}
}

func (e *ELNode) EthClient() *ethclient.Client {
	return ethclient.NewClient(e.RPC())
}

type Eth1Node struct {
	ELNode
	eng     *L1EngineClient
	engLock sync.Mutex
}

func (e *Eth1Node) EngClient() *L1EngineClient {
	e.engLock.Lock()
	defer e.engLock.Unlock()
	if e.eng == nil {
		e.eng = &L1EngineClient{client: e.RPC()}
	}
	return e.eng
}

func finalizedAndSafe(ctx context.Context, cl *ethclient.Client) (*types.Block, *types.Block, error) {
	finalized, err := cl.BlockByNumber(ctx, big.NewInt(int64(rpc.FinalizedBlockNumber)))
	if err == ethereum.NotFound || strings.Contains(err.Error(), "finalized block not found") { // geth api bug: not returning nil error on not found (which would be translated into ethereum.NotFound in bindings)
		genesis, err := cl.BlockByNumber(ctx, big.NewInt(0))
		if err != nil {
			return nil, nil, fmt.Errorf("failed to retrieve genesis: %w", err)
		}
		finalized = genesis
	} else if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch finalized block: %s", err)
	}

	safe, err := cl.BlockByNumber(ctx, big.NewInt(int64(rpc.SafeBlockNumber)))
	if err == ethereum.NotFound || strings.Contains(err.Error(), "safe block not found") { // geth api bug: not returning nil error on not found (which would be translated into ethereum.NotFound in bindings)
		safe = finalized
	} else if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch safe block: %s", err)
	}
	return finalized, safe, nil
}

func (e *Eth1Node) SyncBlocksFrom(ctx context.Context, src *Eth1Node) error {
	destEng := e.EngClient()
	destClient := e.EthClient()
	srcClient := e.EthClient()

	head, err := srcClient.BlockByNumber(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to fetch head block of source node: %w", err)
	}

	finalized, safe, err := finalizedAndSafe(ctx, destClient)
	if err != nil {
		return err
	}

	for {
		got, err := destClient.HeaderByNumber(ctx, head.Number())
		if err != nil && !errors.Is(err, ethereum.NotFound) {
			return fmt.Errorf("failed to fetch block %d from dest client to compare: %w", head.NumberU64(), err)
		}
		if got.Hash() == head.Hash() {
			break
		}
		parent, err := srcClient.BlockByHash(ctx, head.Hash())
		if err != nil {
			return fmt.Errorf("failed to fetch parent bock of %s (%d): %w", head.Hash(), head.NumberU64(), err)
		}
		head = parent
	}

	for {
		// sync range
		bl, err := srcClient.BlockByNumber(ctx, new(big.Int).SetUint64(head.NumberU64()+1))
		if err == ethereum.NotFound {
			return nil
		}
		if err != nil {
			return fmt.Errorf("block by number (%d) from source failed: %w", head.NumberU64()+1, err)
		}
		// make sure we are building on the previous block
		if bl.ParentHash() != head.Hash() {
			return fmt.Errorf("source reorged while syncing destination, source: %s, expected: %s, at height %d", bl.ParentHash(), head.Hash(), head.NumberU64())
		}
		// convert block to payload, to use with engine api
		payload, err := eth.BlockAsPayload(bl)
		if err != nil {
			return fmt.Errorf("block %s (%d) is not a valid payload: %w", bl.Hash(), bl.NumberU64(), err)
		}
		// insert payload into geth
		stat, err := destEng.NewPayload(ctx, payload)
		if err != nil {
			return fmt.Errorf("failed to insert new payload %s (%d) into destionation: %w", payload.BlockHash, payload.BlockNumber, err)
		}
		if stat.Status != eth.ExecutionValid {
			return fmt.Errorf("invalid block %s (%d), cannot sync: %w", payload.BlockHash, payload.BlockNumber, eth.NewPayloadErr(payload, stat))
		}
		// update geth forkchoice to reflect new payload
		fcRes, err := destEng.ForkchoiceUpdate(ctx, &eth.ForkchoiceState{
			HeadBlockHash:      payload.BlockHash,
			SafeBlockHash:      safe.Hash(),
			FinalizedBlockHash: finalized.Hash(),
		}, nil)
		if err != nil {
			return fmt.Errorf("failed to update head forkchoice after inserting new payload %s (%d): %w", payload.BlockHash, payload.BlockNumber, err)
		}
		if fcRes.PayloadStatus.Status != eth.ExecutionValid {
			return fmt.Errorf("invalid forkchoice update: %w", eth.ForkchoiceUpdateErr(fcRes.PayloadStatus))
		}
		// move to next block
		head = bl
	}
}

type L1BlockBuildOpts struct {
	ReorgDepth   uint64
	TimeDelta    uint64
	BuildingTime time.Duration
}

func (e *Eth1Node) BuildBlock(ctx context.Context, opts L1BlockBuildOpts) (common.Hash, error) {
	self := e.EthClient()
	head, err := self.HeaderByNumber(ctx, big.NewInt(int64(rpc.LatestBlockNumber)))
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to get head block: %w", err)
	}
	headNum := head.Number.Uint64()
	if opts.ReorgDepth > headNum {
		return common.Hash{}, fmt.Errorf("cannot reorg %d deep, head is at block height %d", opts.ReorgDepth, headNum)
	}
	finalized, safe, err := finalizedAndSafe(ctx, self)
	if err != nil {
		return common.Hash{}, err
	}

	baseNum := headNum - opts.ReorgDepth
	base, err := self.HeaderByNumber(ctx, big.NewInt(int64(baseNum)))
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to get reorg base block: %w", err)
	}
	var fakeRandao [8]byte
	binary.BigEndian.PutUint64(fakeRandao[:], base.Number.Uint64()+1)
	fcRes, err := e.eng.ForkchoiceUpdate(ctx, &eth.ForkchoiceState{
		HeadBlockHash:      base.Hash(),
		SafeBlockHash:      safe.Hash(),
		FinalizedBlockHash: finalized.Hash(),
	}, &eth.PayloadAttributes{
		Timestamp:             eth.Uint64Quantity(base.Time + opts.TimeDelta),
		PrevRandao:            eth.Bytes32(crypto.Keccak256Hash(fakeRandao[:])),
		SuggestedFeeRecipient: base.Coinbase,
		Transactions:          nil,
		NoTxPool:              false,
	})
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to build block, with rpc error: %w", err)
	} else if fcRes.PayloadStatus.Status != eth.ExecutionValid {
		return common.Hash{}, fmt.Errorf("failed to build block, with engine error: %w", eth.ForkchoiceUpdateErr(fcRes.PayloadStatus))
	}
	time.Sleep(opts.BuildingTime)
	payload, err := e.eng.GetPayload(ctx, *fcRes.PayloadID)
	if err != nil {
		return common.Hash{}, err
	}
	fcRes, err = e.eng.ForkchoiceUpdate(ctx, &eth.ForkchoiceState{
		HeadBlockHash:      payload.BlockHash,
		SafeBlockHash:      safe.Hash(),
		FinalizedBlockHash: finalized.Hash(),
	}, nil)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to persist new block, with rpc error: %w", err)
	} else if fcRes.PayloadStatus.Status != eth.ExecutionValid {
		return common.Hash{}, fmt.Errorf("failed to persist new block, with engine error: %w", eth.ForkchoiceUpdateErr(fcRes.PayloadStatus))
	}
	return payload.BlockHash, nil
}

type OpContracts struct {
	*hivesim.Client
}

// OpL2Engine extends ELNode since it has all the same endpoints, except it is used for L2
type OpL2Engine struct {
	ELNode
}

type OpNode struct {
	*hivesim.Client
}

func (e *OpNode) HttpRpcEndpoint() string {
	return fmt.Sprintf("http://%v:%d", e.IP, RollupRPCPort)
}

func (e *OpNode) RollupClient() *rollupclient.RollupClient {
	return rollupclient.NewRollupClient(e.RPC())
}

func (e *OpNode) P2PClient() *p2p.Client {
	return p2p.NewClient(e.RPC())
}

type ProposerNode struct {
	*hivesim.Client
}

type BatcherNode struct {
	*hivesim.Client
}
