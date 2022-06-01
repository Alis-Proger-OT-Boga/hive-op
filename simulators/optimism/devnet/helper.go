package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/hive/hivesim"
	"github.com/kr/pretty"
)

// default timeout for RPC calls
var rpcTimeout = 10 * time.Second

// TestClient is the environment of a single test.
type TestEnv struct {
	*hivesim.T
	RPC   *rpc.Client
	L1Eth *ethclient.Client
	L2Eth *ethclient.Client
	Vault *vault

	genesis                []byte
	depositContractAddr    common.Address
	withdrawalContractAddr common.Address
	l2OOContractAddr       common.Address

	// This holds most recent context created by the Ctx method.
	// Every time Ctx is called, it creates a new context with the default
	// timeout and cancels the previous one.
	lastCtx    context.Context
	lastCancel context.CancelFunc
}

// runHTTP runs the given test function using the HTTP RPC client.
func runHTTP(t *hivesim.T, c *hivesim.Client, v *vault, g []byte, fn func(*TestEnv)) {
	// This sets up debug logging of the requests and responses.
	client := &http.Client{
		Transport: &loggingRoundTrip{
			t:     t,
			inner: http.DefaultTransport,
		},
	}

	l1RpcClient, _ := rpc.DialHTTPWithClient(fmt.Sprintf("http://%v:8545/", "172.17.0.3"), client)
	defer l1RpcClient.Close()

	// FIXME: hardcoded IP
	l2RpcClient, _ := rpc.DialHTTPWithClient(fmt.Sprintf("http://%v:9545/", c.IP), client)
	defer l2RpcClient.Close()

	env := &TestEnv{
		T:                      t,
		RPC:                    l1RpcClient,
		L1Eth:                  ethclient.NewClient(l1RpcClient),
		L2Eth:                  ethclient.NewClient(l2RpcClient),
		Vault:                  v,
		genesis:                g,
		depositContractAddr:    common.HexToAddress("0xe7f1725E7734CE288F8367e1Bb143E90bb3F0512"),
		withdrawalContractAddr: common.HexToAddress("0x4200000000000000000000000000000000000016"),
		l2OOContractAddr:       common.HexToAddress("0x5FbDB2315678afecb367f032d93F642f64180aa3"),
	}
	fn(env)
	if env.lastCtx != nil {
		env.lastCancel()
	}
}

// runWS runs the given test function using the WebSocket RPC client.
func runWS(t *hivesim.T, c *hivesim.Client, v *vault, g []byte, fn func(*TestEnv)) {
	ctx, done := context.WithTimeout(context.Background(), 5*time.Second)
	l1RpcClient, err := rpc.DialWebsocket(ctx, fmt.Sprintf("ws://%v:8546/", "172.17.0.3"), "")
	done()
	if err != nil {
		t.Fatal("WebSocket connection failed:", err)
	}
	defer l1RpcClient.Close()

	// FIXME: hardcoded IP
	ctx, done = context.WithTimeout(context.Background(), 5*time.Second)
	l2RpcClient, err := rpc.DialWebsocket(ctx, fmt.Sprintf("ws://%v:9546/", c.IP), "")
	done()
	if err != nil {
		t.Fatal("WebSocket connection failed:", err)
	}
	defer l2RpcClient.Close()

	env := &TestEnv{
		T:                      t,
		RPC:                    l1RpcClient,
		L1Eth:                  ethclient.NewClient(l1RpcClient),
		L2Eth:                  ethclient.NewClient(l2RpcClient),
		Vault:                  v,
		genesis:                g,
		depositContractAddr:    common.HexToAddress("0xe7f1725E7734CE288F8367e1Bb143E90bb3F0512"),
		withdrawalContractAddr: common.HexToAddress("0x4200000000000000000000000000000000000016"),
		l2OOContractAddr:       common.HexToAddress("0x5FbDB2315678afecb367f032d93F642f64180aa3"),
	}
	fn(env)
	if env.lastCtx != nil {
		env.lastCancel()
	}
}

// CallContext is a helper method that forwards a raw RPC request to
// the underlying RPC client. This can be used to call RPC methods
// that are not supported by the ethclient.Client.
func (t *TestEnv) CallContext(ctx context.Context, result interface{}, method string, args ...interface{}) error {
	return t.RPC.CallContext(ctx, result, method, args...)
}

// LoadGenesis returns the genesis block.
func (t *TestEnv) LoadGenesis() *types.Block {
	var genesis core.Genesis
	if err := json.Unmarshal(t.genesis, &genesis); err != nil {
		panic(fmt.Errorf("can't parse genesis JSON: %v", err))
	}
	return genesis.ToBlock(nil)
}

// Ctx returns a context with the default timeout.
// For subsequent calls to Ctx, it also cancels the previous context.
func (t *TestEnv) Ctx() context.Context {
	if t.lastCtx != nil {
		t.lastCancel()
	}
	t.lastCtx, t.lastCancel = context.WithTimeout(context.Background(), rpcTimeout)
	return t.lastCtx
}

func waitSynced(c *rpc.Client) (err error) {
	var (
		timeout     = 20 * time.Second
		end         = time.Now().Add(timeout)
		ctx, cancel = context.WithDeadline(context.Background(), end)
	)
	defer func() {
		cancel()
		if err == context.DeadlineExceeded {
			err = fmt.Errorf("didn't sync within timeout of %v", 20*time.Second)
		}
	}()

	ec := ethclient.NewClient(c)
	for {
		progress, err := ec.SyncProgress(ctx)
		if err != nil {
			return err
		}
		head, err := ec.BlockNumber(ctx)
		if err != nil {
			return err
		}
		if progress == nil && head > 0 {
			return nil // success!
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// Naive generic function that works in all situations.
// A better solution is to use logs to wait for confirmations.
func waitForTxConfirmations(t *TestEnv, txHash common.Hash, n uint64) (*types.Receipt, error) {
	var (
		receipt    *types.Receipt
		startBlock *types.Block
		err        error
	)

	for i := 0; i < 90; i++ {
		receipt, err = t.L2Eth.TransactionReceipt(t.Ctx(), txHash)
		if err != nil && err != ethereum.NotFound {
			return nil, err
		}
		if receipt != nil {
			break
		}
		time.Sleep(time.Second)
	}
	if receipt == nil {
		return nil, ethereum.NotFound
	}

	if startBlock, err = t.L2Eth.BlockByNumber(t.Ctx(), nil); err != nil {
		return nil, err
	}

	for i := 0; i < 90; i++ {
		currentBlock, err := t.L2Eth.BlockByNumber(t.Ctx(), nil)
		if err != nil {
			return nil, err
		}

		if startBlock.NumberU64()+n >= currentBlock.NumberU64() {
			if checkReceipt, err := t.L2Eth.TransactionReceipt(t.Ctx(), txHash); checkReceipt != nil {
				if bytes.Compare(receipt.PostState, checkReceipt.PostState) == 0 {
					return receipt, nil
				} else { // chain reorg
					waitForTxConfirmations(t, txHash, n)
				}
			} else {
				return nil, err
			}
		}

		time.Sleep(time.Second)
	}

	return nil, ethereum.NotFound
}

func waitForTransaction(hash common.Hash, client *ethclient.Client, timeout time.Duration) (*types.Receipt, error) {
	timeoutCh := time.After(timeout)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	for {
		receipt, err := client.TransactionReceipt(ctx, hash)
		if receipt != nil && err == nil {
			return receipt, nil
		} else if err != nil && !errors.Is(err, ethereum.NotFound) {
			return nil, err
		}

		select {
		case <-timeoutCh:
			return nil, errors.New("timeout")
		case <-time.After(100 * time.Millisecond):
		}
	}
}

// loggingRoundTrip writes requests and responses to the test log.
type loggingRoundTrip struct {
	t     *hivesim.T
	inner http.RoundTripper
}

func (rt *loggingRoundTrip) RoundTrip(req *http.Request) (*http.Response, error) {
	// Read and log the request body.
	reqBytes, err := ioutil.ReadAll(req.Body)
	req.Body.Close()
	if err != nil {
		return nil, err
	}
	rt.t.Logf(">>  %s", bytes.TrimSpace(reqBytes))
	reqCopy := *req
	reqCopy.Body = ioutil.NopCloser(bytes.NewReader(reqBytes))

	// Do the round trip.
	resp, err := rt.inner.RoundTrip(&reqCopy)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Read and log the response bytes.
	respBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	respCopy := *resp
	respCopy.Body = ioutil.NopCloser(bytes.NewReader(respBytes))
	rt.t.Logf("<<  %s", bytes.TrimSpace(respBytes))
	return &respCopy, nil
}

// diff checks whether x and y are deeply equal, returning a description
// of their differences if they are not equal.
func diff(x, y interface{}) (d string) {
	for _, l := range pretty.Diff(x, y) {
		d += l + "\n"
	}
	return d
}
