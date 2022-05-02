package main

import (
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
)

var (
	big0 = new(big.Int)
	big1 = big.NewInt(1)
)

// genesisByHash fetches the known genesis header and compares
// it against the genesis file to determine if block fields are
// returned correct.
func genesisHeaderByHashTest(t *TestEnv) {
	gblock := t.LoadGenesis()

	headerByHash, err := t.Eth.HeaderByHash(t.Ctx(), gblock.Hash())
	if err != nil {
		t.Fatalf("Unable to fetch block %x: %v", gblock.Hash(), err)
	}
	if d := diff(gblock.Header(), headerByHash); d != "" {
		t.Fatal("genesis header reported by node differs from expected header:\n", d)
	}
}

// headerByNumberTest fetched the known genesis header and compares
// it against the genesis file to determine if block fields are
// returned correct.
func genesisHeaderByNumberTest(t *TestEnv) {
	gblock := t.LoadGenesis()

	headerByNum, err := t.Eth.HeaderByNumber(t.Ctx(), big0)
	if err != nil {
		t.Fatalf("Unable to fetch genesis block: %v", err)
	}
	if d := diff(gblock.Header(), headerByNum); d != "" {
		t.Fatal("genesis header reported by node differs from expected header:\n", d)
	}
}

// genesisBlockByHashTest fetched the known genesis block and compares it against
// the genesis file to determine if block fields are returned correct.
func genesisBlockByHashTest(t *TestEnv) {
	gblock := t.LoadGenesis()

	blockByHash, err := t.Eth.BlockByHash(t.Ctx(), gblock.Hash())
	if err != nil {
		t.Fatalf("Unable to fetch block %x: %v", gblock.Hash(), err)
	}
	if d := diff(gblock.Header(), blockByHash.Header()); d != "" {
		t.Fatal("genesis header reported by node differs from expected header:\n", d)
	}
}

// genesisBlockByNumberTest retrieves block 0 since that is the only block
// that is known through the genesis.json file and tests if block
// fields matches the fields defined in the genesis file.
func genesisBlockByNumberTest(t *TestEnv) {
	gblock := t.LoadGenesis()

	blockByNum, err := t.Eth.BlockByNumber(t.Ctx(), big0)
	if err != nil {
		t.Fatalf("Unable to fetch genesis block: %v", err)
	}
	if d := diff(gblock.Header(), blockByNum.Header()); d != "" {
		t.Fatal("genesis header reported by node differs from expected header:\n", d)
	}
}

// syncProgressTest only tests if this function is supported by the node.
func syncProgressTest(t *TestEnv) {
	_, err := t.Eth.SyncProgress(t.Ctx())
	if err != nil {
		t.Fatalf("Unable to determine sync progress: %v", err)
	}
}

// newHeadSubscriptionTest tests whether
func newHeadSubscriptionTest(t *TestEnv) {
	var (
		heads = make(chan *types.Header)
	)

	sub, err := t.Eth.SubscribeNewHead(t.Ctx(), heads)
	if err != nil {
		t.Fatalf("Unable to subscribe to new heads: %v", err)
	}

	defer sub.Unsubscribe()
	for i := 0; i < 10; i++ {
		select {
		case newHead := <-heads:
			header, err := t.Eth.HeaderByHash(t.Ctx(), newHead.Hash())
			if err != nil {
				t.Fatalf("Unable to fetch header: %v", err)
			}
			if header == nil {
				t.Fatalf("Unable to fetch header %s", newHead.Hash())
			}
		case err := <-sub.Err():
			t.Fatalf("Received errors: %v", err)
		}
	}
}

// balanceAndNonceAtTest creates a new account and transfers funds to it.
// It then tests if the balance and nonce of the sender and receiver
// address are updated correct.
func balanceAndNonceAtTest(t *TestEnv) {
	var (
		sourceAddr  = t.Vault.createAccount(t, big.NewInt(params.Ether))
		sourceNonce = uint64(0)
		targetAddr  = t.Vault.createAccount(t, nil)
	)

	// Get current balance
	sourceAddressBalanceBefore, err := t.Eth.BalanceAt(t.Ctx(), sourceAddr, nil)
	if err != nil {
		t.Fatalf("Unable to retrieve balance: %v", err)
	}

	expected := big.NewInt(params.Ether)
	if sourceAddressBalanceBefore.Cmp(expected) != 0 {
		t.Errorf("Expected balance %d, got %d", expected, sourceAddressBalanceBefore)
	}

	nonceBefore, err := t.Eth.NonceAt(t.Ctx(), sourceAddr, nil)
	if err != nil {
		t.Fatalf("Unable to determine nonce: %v", err)
	}
	if nonceBefore != sourceNonce {
		t.Fatalf("Invalid nonce, want %d, got %d", sourceNonce, nonceBefore)
	}

	// send 1234 wei to target account and verify balances and nonces are updated
	var (
		amount   = big.NewInt(1234)
		gasLimit = uint64(50000)
	)
	rawTx := types.NewTransaction(sourceNonce, targetAddr, amount, gasLimit, gasPrice, nil)
	valueTx, err := t.Vault.signTransaction(sourceAddr, rawTx)
	if err != nil {
		t.Fatalf("Unable to sign value tx: %v", err)
	}
	sourceNonce++

	t.Logf("BalanceAt: send %d wei from 0x%x to 0x%x in 0x%x", valueTx.Value(), sourceAddr, targetAddr, valueTx.Hash())
	if err := t.Eth.SendTransaction(t.Ctx(), valueTx); err != nil {
		t.Fatalf("Unable to send transaction: %v", err)
	}

	var receipt *types.Receipt
	for {
		receipt, err = t.Eth.TransactionReceipt(t.Ctx(), valueTx.Hash())
		if receipt != nil {
			break
		}
		if err != ethereum.NotFound {
			t.Fatalf("Could not fetch receipt for 0x%x: %v", valueTx.Hash(), err)
		}
		time.Sleep(time.Second)
	}

	// ensure balances have been updated
	accountBalanceAfter, err := t.Eth.BalanceAt(t.Ctx(), sourceAddr, nil)
	if err != nil {
		t.Fatalf("Unable to retrieve balance: %v", err)
	}
	balanceTargetAccountAfter, err := t.Eth.BalanceAt(t.Ctx(), targetAddr, nil)
	if err != nil {
		t.Fatalf("Unable to retrieve balance: %v", err)
	}

	// expected balance is previous balance - tx amount - tx fee (gasUsed * gasPrice)
	exp := new(big.Int).Set(sourceAddressBalanceBefore)
	exp.Sub(exp, amount)
	exp.Sub(exp, new(big.Int).Mul(big.NewInt(int64(receipt.GasUsed)), valueTx.GasPrice()))

	if exp.Cmp(accountBalanceAfter) != 0 {
		t.Errorf("Expected sender account to have a balance of %d, got %d", exp, accountBalanceAfter)
	}
	if balanceTargetAccountAfter.Cmp(amount) != 0 {
		t.Errorf("Expected new account to have a balance of %d, got %d", valueTx.Value(), balanceTargetAccountAfter)
	}

	// ensure nonce is incremented by 1
	nonceAfter, err := t.Eth.NonceAt(t.Ctx(), sourceAddr, nil)
	if err != nil {
		t.Fatalf("Unable to determine nonce: %v", err)
	}
	expectedNonce := nonceBefore + 1
	if expectedNonce != nonceAfter {
		t.Fatalf("Invalid nonce, want %d, got %d", expectedNonce, nonceAfter)
	}
}
