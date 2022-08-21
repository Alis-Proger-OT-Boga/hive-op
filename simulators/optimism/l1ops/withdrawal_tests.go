package main

import (
	"encoding/json"
	"fmt"
	"github.com/ethereum-optimism/optimism/op-bindings/bindings"
	"github.com/ethereum-optimism/optimism/op-bindings/predeploys"
	"github.com/ethereum-optimism/optimism/op-node/withdrawals"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/optimism"
	"github.com/stretchr/testify/require"
	"math/big"
	"time"
)

func simpleWithdrawalTest(t *hivesim.T, env *optimism.TestEnv) {
	l1 := env.Devnet.L1Client(0)
	l2 := env.Devnet.L2Client(0)
	l1Vault := env.Devnet.L1Vault
	l2Vault := env.Devnet.L2Vault
	depositor := l1Vault.CreateAccount(env.TimeoutCtx(time.Minute), l1, big.NewInt(params.Ether))
	l2Vault.InsertKey(l1Vault.FindKey(depositor))

	mintAmount := big.NewInt(0.5 * params.Ether)
	doDeposit(t, env, depositor, mintAmount)
	//startBalanceL1, err := l1.BalanceAt(env.Ctx(), depositor, nil)
	//require.NoError(t, err)
	startBalanceL2, err := l2.BalanceAt(env.Ctx(), depositor, nil)
	require.NoError(t, err)
	require.EqualValues(t, mintAmount, startBalanceL2)

	l2Wd, err := bindings.NewL2ToL1MessagePasser(predeploys.L2ToL1MessagePasserAddr, l2)
	require.Nil(t, err, "binding withdrawer on L2")

	withdrawAmount := big.NewInt(0.25 * params.Ether)
	l2Opts := l2Vault.KeyedTransactor(depositor)
	l2Opts.Value = withdrawAmount
	initTx, err := l2Wd.InitiateWithdrawal(l2Opts, depositor, big.NewInt(21000), nil)
	require.NoError(t, err)
	initReceipt, err := optimism.WaitReceipt(env.TimeoutCtx(time.Minute), l2, initTx.Hash())
	require.NoError(t, err)
	require.Equal(t, initReceipt.Status, types.ReceiptStatusSuccessful)
	//initHeader, err := l2.HeaderByNumber(env.Ctx(), initReceipt.BlockNumber)
	//require.NoError(t, err)
	//endBalanceL2, err := l2.BalanceAt(env.Ctx(), depositor, initReceipt.BlockNumber)
	//require.NoError(t, err)

	finBlockNum, err := withdrawals.WaitForFinalizationPeriod(
		env.TimeoutCtx(5*time.Minute),
		l1,
		env.Devnet.Deployments.OptimismPortalProxy,
		initReceipt.BlockNumber,
	)
	require.NoError(t, err)
	finHeader, err := l2.HeaderByNumber(env.Ctx(), big.NewInt(int64(finBlockNum)))
	require.NoError(t, err)

	// Take fee into account
	//diff := new(big.Int).Sub(startBalanceL2, endBalanceL2)
	//fees := calcGasFees(initReceipt.GasUsed, initTx.GasTipCap(), initTx.GasFeeCap(), initHeader.BaseFee)
	//diff = diff.Sub(diff, fees)
	//require.Equal(t, withdrawAmount, diff)

	l2Client := withdrawals.NewClient(env.Devnet.GetOpL2Engine(0).RPC())
	wParams, err := withdrawals.FinalizeWithdrawalParameters(env.Ctx(), l2Client, initTx.Hash(), finHeader)
	require.NoError(t, err)

	portal, err := bindings.NewOptimismPortal(
		env.Devnet.Deployments.OptimismPortalProxy,
		l1,
	)
	require.NoError(t, err)

	l1Opts := l1Vault.KeyedTransactor(depositor)
	require.NoError(t, err)
	finTx, err := portal.FinalizeWithdrawalTransaction(
		l1Opts,
		bindings.TypesWithdrawalTransaction{
			Nonce:    wParams.Nonce,
			Sender:   wParams.Sender,
			Target:   wParams.Target,
			Value:    wParams.Value,
			GasLimit: wParams.GasLimit,
			Data:     wParams.Data,
		},
		wParams.BlockNumber,
		wParams.OutputRootProof,
		wParams.WithdrawalProof,
	)
	require.NoError(t, err)

	finReceipt, err := optimism.WaitReceipt(env.TimeoutCtx(time.Minute), l1, finTx.Hash())
	require.NoError(t, err)
	require.Equal(t, types.ReceiptStatusSuccessful, finReceipt.Status)

	//endBalanceL1, err := l1.BalanceAt(env.Ctx(), depositor, nil)
	//require.NoError(t, err)

	//diff = new(big.Int).Sub(endBalanceL1, startBalanceL1)
	//fees = calcGasFees(initReceipt.GasUsed, initTx.GasTipCap(), initTx.GasFeeCap(), finHeader.BaseFee)
	//withdrawAmount = withdrawAmount.Sub(withdrawAmount, fees)
	//require.Equal(t, withdrawAmount, diff)
}

// calcGasFees determines the actual cost of the transaction given a specific basefee
func calcGasFees(gasUsed uint64, gasTipCap *big.Int, gasFeeCap *big.Int, baseFee *big.Int) *big.Int {
	x := new(big.Int).Add(gasTipCap, baseFee)
	// If tip + basefee > gas fee cap, clamp it to the gas fee cap
	if x.Cmp(gasFeeCap) > 0 {
		x = gasFeeCap
	}
	return x.Mul(x, new(big.Int).SetUint64(gasUsed))
}
