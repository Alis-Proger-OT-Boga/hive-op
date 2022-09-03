package optimism

import (
	"encoding/json"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"testing"
)

func TestHardhat(t *testing.T) {
	c := HardhatDeployConfig{
		L1StartingBlockTag: "latest",
		L1ChainID:          uint64(L1ChainID),
		L2ChainID:          uint64(L2ChainID),
		L2BlockTime:        2,

		MaxSequencerDrift:      20,
		SequencerWindowSize:    100,
		ChannelTimeout:         30,
		P2PSequencerAddress:    common.Address{},
		OptimismL2FeeRecipient: common.Address{0: 0x42, 19: 0xf0}, // tbd
		BatchInboxAddress:      common.Address{0: 0x42, 19: 0xff}, // tbd
		BatchSenderAddress:     common.Address{},

		L2OutputOracleSubmissionInterval: 6,
		L2OutputOracleStartingTimestamp:  -1,
		L2OutputOracleProposer:           common.Address{},
		L2OutputOracleOwner:              common.Address{}, // tbd

		L1BlockTime:                 15,
		L1GenesisBlockNonce:         0,
		CliqueSignerAddress:         common.Address{},
		L1GenesisBlockGasLimit:      15_000_000,
		L1GenesisBlockDifficulty:    1,
		L1GenesisBlockMixHash:       common.Hash{},
		L1GenesisBlockCoinbase:      common.Address{},
		L1GenesisBlockNumber:        0,
		L1GenesisBlockGasUsed:       0,
		L1GenesisBlockParentHash:    common.Hash{},
		L1GenesisBlockBaseFeePerGas: 1000_000_000, // 1 gwei

		L2GenesisBlockNonce:         0,
		L2GenesisBlockExtraData:     []byte{},
		L2GenesisBlockGasLimit:      15_000_000,
		L2GenesisBlockDifficulty:    1,
		L2GenesisBlockMixHash:       common.Hash{},
		L2GenesisBlockCoinbase:      common.Address{0: 0x42, 19: 0xf0}, // matching OptimismL2FeeRecipient
		L2GenesisBlockNumber:        0,
		L2GenesisBlockGasUsed:       0,
		L2GenesisBlockParentHash:    common.Hash{},
		L2GenesisBlockBaseFeePerGas: 1000_000_000,

		OptimismBaseFeeRecipient:    common.Address{0: 0x42, 19: 0xf1}, // tbd
		OptimismL1FeeRecipient:      common.Address{},
		L2CrossDomainMessengerOwner: common.Address{0: 0x42, 19: 0xf2}, // tbd
		GasPriceOracleOwner:         common.Address{0: 0x42, 19: 0xf3}, // tbd
		GasPriceOracleOverhead:      2100,
		GasPriceOracleScalar:        1000_000,
		GasPriceOracleDecimals:      6,

		ProxyAdmin:                  common.Address{0: 0x42, 19: 0xf4}, // tbd
		FundDevAccounts:             true,
		DeploymentWaitConfirmations: 1,
	}
	j, _ := json.MarshalIndent(c, "", "  ")
	fmt.Println(string(j))
}
