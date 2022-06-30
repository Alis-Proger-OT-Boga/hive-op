package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/ethclient/gethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/hive/hivesim"
	"github.com/stretchr/testify/require"
)

func main() {
	suite := hivesim.Suite{
		Name:        "optimism reorg",
		Description: "This suite runs the reorg protocol tests",
	}

	// Add tests for full nodes.
	suite.Add(&hivesim.TestSpec{
		Name:        "client launch",
		Description: `This test launches the client and collects its logs.`,
		Run:         func(t *hivesim.T) { runReorgTests(t) },
	})

	sim := hivesim.New()
	hivesim.MustRunSuite(sim, suite)
}

// runReorgTests runs the reorg tests.
func runReorgTests(t *hivesim.T) {
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
	d.InitOpSequencer()
	d.StartOpSequencer()
	d.StartOpVerifier()
	d.StartL2OS()
	d.StartBSS()

	l1BlockTime := 15 * time.Second

	time.Sleep(10 * l1BlockTime)

	// This sets up debug logging of the requests and responses.
	client := &http.Client{
		Transport: &loggingRoundTrip{
			t:     t,
			inner: http.DefaultTransport,
		},
	}

	rpcClient, _ := rpc.DialHTTPWithClient(fmt.Sprintf("http://%v:8545/", d.eth1.IP), client)
	defer rpcClient.Close()
	l1Client := ethclient.NewClient(rpcClient)
	l1GethClient := gethclient.New(rpcClient)

	ctx, cancel = context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	l1Header, err := l1Client.HeaderByNumber(ctx, nil)
	require.Nil(t, err)

	fmt.Println("before sethead: %d", l1Header.Number)

	l1GethClient.SetHead(ctx, l1Header.Number)

	time.Sleep(10 * l1BlockTime)

	l1Header, err = l1Client.HeaderByNumber(ctx, nil)
	require.Nil(t, err)

	fmt.Println("after sethead: %d", l1Header.Number)
}
