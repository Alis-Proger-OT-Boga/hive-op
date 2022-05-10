package main

import (
	"context"
	"math/big"
	"time"

	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/hive/hivesim"
)

type testSpec struct {
	Name  string
	About string
	Run   func(*TestEnv)
}

func setup(t *TestEnv) {
	var (
		address = t.Config.Vault.createAccount(t, big.NewInt(params.Ether))
		nonce   = uint64(0)

		expectedContractAddress = crypto.CreateAddress(address, nonce)
		gasLimit                = uint64(1200000)
	)

	rawTx := types.NewContractCreation(nonce, big0, gasLimit, gasPrice, deployCode)
	deployTx, err := t.Config.Vault.signTransaction(address, rawTx)
	if err != nil {
		t.Fatalf("Unable to sign deploy tx: %v", err)
	}

	// deploy contract
	if err := t.Eth.SendTransaction(t.Ctx(), deployTx); err != nil {
		t.Fatalf("Unable to send transaction: %v", err)
	}

	t.Logf("Deploy transaction: 0x%x", deployTx.Hash())

	// fetch transaction receipt for contract address
	receipt, err := waitForTxConfirmations(t, deployTx.Hash(), 5)
	if err != nil {
		t.Fatalf("Unable to retrieve receipt: %v", err)
	}

	// ensure receipt has the expected address
	if expectedContractAddress != receipt.ContractAddress {
		t.Fatalf("Contract deploy on different address, expected %x, got %x", expectedContractAddress, receipt.ContractAddress)
	}

	t.Config.DeployedContractAddr = receipt.ContractAddress
}

var tests = []testSpec{
	// HTTP RPC tests.
	{Name: "http/BalanceAndNonceAt", Run: balanceAndNonceAtTest},
	{Name: "http/CodeAt", Run: CodeAtTest},
	{Name: "http/ContractDeployment", Run: deployContractTest},
	{Name: "http/ContractDeploymentOutOfGas", Run: deployContractOutOfGasTest},
	{Name: "http/GenesisBlockByHash", Run: genesisBlockByHashTest},
	{Name: "http/GenesisBlockByNumber", Run: genesisBlockByNumberTest},
	{Name: "http/GenesisHeaderByHash", Run: genesisHeaderByHashTest},
	{Name: "http/GenesisHeaderByNumber", Run: genesisHeaderByNumberTest},
	{Name: "http/SyncProgress", Run: syncProgressTest},

	// WebSocket RPC tests.
	{Name: "ws/BalanceAndNonceAt", Run: balanceAndNonceAtTest},
	{Name: "ws/ContractDeployment", Run: deployContractTest},
	{Name: "ws/ContractDeploymentOutOfGas", Run: deployContractOutOfGasTest},
	{Name: "ws/GenesisBlockByHash", Run: genesisBlockByHashTest},
	{Name: "ws/GenesisBlockByNumber", Run: genesisBlockByNumberTest},
	{Name: "ws/GenesisHeaderByHash", Run: genesisHeaderByHashTest},
	{Name: "ws/GenesisHeaderByNumber", Run: genesisHeaderByNumberTest},
	{Name: "ws/SyncProgress", Run: syncProgressTest},

	// WebSocket subscription tests.
	{Name: "ws/NewHeadSubscription", Run: newHeadSubscriptionTest},
}

func main() {
	suite := hivesim.Suite{
		Name: "optimism rpc",
		Description: `
The RPC test suite runs a set of RPC related tests against a running node. It tests
several real-world scenarios such as sending value transactions, deploying a contract or
interacting with one.`[1:],
	}

	// Add tests for full nodes.
	suite.Add(&hivesim.TestSpec{
		Name:        "client launch",
		Description: `This test launches the client and collects its logs.`,
		Run:         func(t *hivesim.T) { runAllTests(t) },
	})

	sim := hivesim.New()
	hivesim.MustRunSuite(sim, suite)
}

// runAllTests runs the tests against a client instance.
// Most tests simply wait for tx inclusion in a block so we can run many tests concurrently.
func runAllTests(t *hivesim.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	d := Devnet{
		t:     t,
		nodes: make(map[string]*hivesim.ClientDefinition),
		ctx:   ctx,
	}
	d.Start()
	d.Wait()
	d.DeployL1()
	d.InitL2()
	d.StartL2()
	d.InitOp()
	d.StartOp()
	d.StartL2OS()

	c := d.l2.Client

	// Setup deployed contract
	config := &TestConfig{
		Vault:          newVault(),
		L1GenesisBlock: []byte(d.l2Genesis),
	}
	runHTTP(t, c, config, setup)

	s := newSemaphore(16)
	for _, test := range tests {
		test := test
		s.get()
		go func() {
			defer s.put()
			t.Run(hivesim.TestSpec{
				Name:        fmt.Sprintf("%s (%s)", test.Name, "ops-l2"),
				Description: test.About,
				Run: func(t *hivesim.T) {
					switch test.Name[:strings.IndexByte(test.Name, '/')] {
					case "http":
						runHTTP(t, c, config, test.Run)
					case "ws":
						runWS(t, c, config, test.Run)
					default:
						panic("bad test prefix in name " + test.Name)
					}
				},
			})
		}()
	}
	s.drain()
}

type semaphore chan struct{}

func newSemaphore(n int) semaphore {
	s := make(semaphore, n)
	for i := 0; i < n; i++ {
		s <- struct{}{}
	}
	return s
}

func (s semaphore) get() { <-s }
func (s semaphore) put() { s <- struct{}{} }

func (s semaphore) drain() {
	for i := 0; i < cap(s); i++ {
		<-s
	}
}
