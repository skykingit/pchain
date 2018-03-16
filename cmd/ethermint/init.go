package main

import (
	"os"
	"path/filepath"

	"gopkg.in/urfave/cli.v1"

	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/logger"
	"github.com/ethereum/go-ethereum/logger/glog"

	cmn "github.com/tendermint/go-common"
	"github.com/tendermint/tendermint/types"
	"github.com/ethereum/go-ethereum/common"
	"math/big"
	"github.com/pkg/errors"
	"io/ioutil"
	"encoding/json"
	"fmt"
	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/console"
	"time"
)

func initCmd(ctx *cli.Context) error {

	// ethereum genesis.json
	//ethGenesisPath := ctx.Args().First()
	//if len(ethGenesisPath) == 0 {
	//	utils.Fatalf("must supply path to genesis JSON file")
	//}
	coreGensis, privValidator, err := init_eth_genesis()
	if err != nil {
		utils.Fatalf("init eth_genesis failed")
		return err
	}
	init_eth_blockchain(config.GetString("eth_genesis_file"), ctx)

	init_em_files(*coreGensis, privValidator)
	/*
	rsTmplPath := ctx.Args().Get(1)
	init_reward_scheme_files(rsTmplPath)
	*/
	return nil
}

func init_eth_blockchain(ethGenesisPath string, ctx *cli.Context) {

	chainDb, err := ethdb.NewLDBDatabase(filepath.Join(utils.MakeDataDir(ctx), "chaindata"), 0, 0)
	if err != nil {
		utils.Fatalf("could not open database: %v", err)
	}

	genesisFile, err := os.Open(ethGenesisPath)
	if err != nil {
		utils.Fatalf("failed to read genesis file: %v", err)
	}

	block, err := core.WriteGenesisBlock(chainDb, genesisFile)
	if err != nil {
		utils.Fatalf("failed to write genesis block: %v", err)
	}

	glog.V(logger.Info).Infof("successfully wrote genesis block and/or chain rule set: %x", block.Hash())
}

func init_em_files(coreGenesis core.Genesis, privValidator *types.PrivValidator) {

	// if no genesis, make it using the priv val
	genFile := config.GetString("genesis_file")
	if _, err := os.Stat(genFile); os.IsNotExist(err) {
		genDoc := types.GenesisDoc{
			ChainID: cmn.Fmt("test-chain-%v", cmn.RandStr(6)),
			Consensus: types.CONSENSUS_POS,
			RewardScheme: types.RewardSchemeDoc {
  				TTotalReward :     "210000000000000000000000000",
                PreAllocated :    "178500000000000000000000000",
				AddedPerYear :    "0",
				RewardFirstYear :   "5727300000000000000000000",
				DescendPerYear :     "572730000000000000000000",
				Allocated : "0",
				EpochNumberPerYear: "525600",
			},
			CurrentEpoch: types.OneEpochDoc{
				Number :		"0",
				RewardPerBlock :	"1666666666666666666666667",
				StartBlock :		"0",
				EndBlock :		"1295999",
				StartTime :		time.Now().Format(time.RFC3339Nano),
				EndTime :		"0",//not accurate for current epoch
				BlockGenerated :	"0",
				Status :		"0",
			},
		}

		coinbase, amount, checkErr := checkAccount(coreGenesis)
		if(checkErr != nil) {
			glog.V(logger.Error).Infof(checkErr.Error())
			cmn.Exit(checkErr.Error())
		}

		genDoc.CurrentEpoch.Validators = []types.GenesisValidator{types.GenesisValidator{
			EthAccount: coinbase,
			PubKey: privValidator.PubKey,
			Amount: amount,
		}}
		genDoc.SaveAs(genFile)
	}

}

func init_eth_genesis() (*core.Genesis, *types.PrivValidator,error) {
	privValFile := config.GetString("priv_validator_file")
	var privValidator *types.PrivValidator
	var newKey *keystore.Key
	if _, err := os.Stat(privValFile); os.IsNotExist(err) {
		privValidator,newKey = types.GenPrivValidatorKey()
		privValidator.SetFile(privValFile)
		privValidator.Save()
		scryptN := keystore.StandardScryptN
		scryptP := keystore.StandardScryptP
		password := getPassPhrase("Your new account is locked with a password. Please give a password. Do not forget this password.", true)
		ks := keystore.NewKeyStoreByTenermint(config.GetString("keystore"), scryptN, scryptP)

		a := accounts.Account{Address: newKey.Address, URL: accounts.URL{Scheme: keystore.KeyStoreScheme, Path: ks.Ks.JoinPath(keystore.KeyFileName(newKey.Address))}}
		if err := ks.StoreKey(a.URL.Path, newKey, password); err != nil {
			utils.Fatalf("store key failed")
			return nil, nil, err
		}
	} else {
		if _, err := os.Stat(privValFile); err != nil {
			if os.IsNotExist(err) {
				utils.Fatalf("failed to read eth_genesis file: %v", err)
			}
			return nil, nil, err
		}
		privValidator = types.LoadOrGenPrivValidator(privValFile)
	}
	var coreGenesis = core.Genesis{
		Nonce: "0xdeadbeefdeadbeef",
		Timestamp: "0x0",
		ParentHash: "0x0000000000000000000000000000000000000000000000000000000000000000",
		ExtraData: "0x0",
		GasLimit: "0x8000000",
		Difficulty: "0x400",
		Mixhash: "0x0000000000000000000000000000000000000000000000000000000000000000",
		Coinbase: common.ToHex((*privValidator).Address),
		Alloc: map[string]struct {
			Code    string
			Storage map[string]string
			Balance string
			Nonce   string
		}{
			common.ToHex((*privValidator).Address):{Balance: "10000000000000000000000000000000000" },
		},
	}
	contents, err := json.Marshal(coreGenesis)
	if err != nil {
		utils.Fatalf("marshal coreGenesis failed")
		return nil, nil, err
	}
	ethGenesisPath := config.GetString("eth_genesis_file")
	if err = ioutil.WriteFile(ethGenesisPath, contents, 0654); err != nil {
		utils.Fatalf("write eth_genesis_file failed")
		return nil, nil, err
	}
	return &coreGenesis, privValidator, nil
}


func checkAccount(coreGenesis core.Genesis) (common.Address, int64, error) {

	coinbase := common.HexToAddress(coreGenesis.Coinbase)
	amount := int64(10)
	balance := big.NewInt(-1)
	found := false
	fmt.Printf("checkAccount(), coinbase is %x\n", coinbase)
	for addr, account := range coreGenesis.Alloc {
		address := common.HexToAddress(addr)
		fmt.Printf("checkAccount(), address is %x\n", address)
		if coinbase == address {
			balance = common.String2Big(account.Balance)
			found = true
			break
		}
	}

	if( !found ) {
		fmt.Printf("invalidate eth_account\n")
		return common.Address{}, int64(0), errors.New("invalidate eth_account")
	}

	if ( balance.Cmp(big.NewInt(amount)) < 0) {
		fmt.Printf("balance is not enough to be support validator's amount, balance is %v, amount is %v\n",
			balance, amount)
		return common.Address{}, int64(0), errors.New("no enough balance")
	}

	return coinbase, amount, nil
}

// getPassPhrase retrieves the passwor associated with an account, either fetched
// from a list of preloaded passphrases, or requested interactively from the user.
func getPassPhrase(prompt string, confirmation bool) string {
	//prompt the user for the password
	if prompt != "" {
		fmt.Println(prompt)
	}
	password, err := console.Stdin.PromptPassword("Passphrase: ")
	if err != nil {
		utils.Fatalf("Failed to read passphrase: %v", err)
	}
	if confirmation {
		confirm, err := console.Stdin.PromptPassword("Repeat passphrase: ")
		if err != nil {
			utils.Fatalf("Failed to read passphrase confirmation: %v", err)
		}
		if password != confirm {
			utils.Fatalf("Passphrases do not match")
		}
	}
	return password
}



