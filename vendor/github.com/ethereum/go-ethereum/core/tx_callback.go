package core

import (
	"errors"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/tendermint/epoch"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	pabi "github.com/pchain/abi"
	dbm "github.com/tendermint/go-db"
	"math/big"
	"sync"
)

type CrossChainHelper interface {
	GetMutex() *sync.Mutex
	GetClient() *ethclient.Client
	GetChainInfoDB() dbm.DB

	CanCreateChildChain(from common.Address, chainId string, minValidators uint16, minDepositAmount *big.Int, startBlock, endBlock *big.Int) error
	CreateChildChain(from common.Address, chainId string, minValidators uint16, minDepositAmount *big.Int, startBlock, endBlock *big.Int) error
	ValidateJoinChildChain(from common.Address, pubkey string, chainId string, depositAmount *big.Int) error
	JoinChildChain(from common.Address, pubkey string, chainId string, depositAmount *big.Int) error
	ReadyForLaunchChildChain(height *big.Int, stateDB *state.StateDB) ([]string, []byte, []string)
	ProcessPostPendingData(newPendingIdxBytes []byte, deleteChildChainIds []string)

	ValidateVoteNextEpoch(chainId string) error
	VoteNextEpoch(ep *epoch.Epoch, from common.Address, voteHash common.Hash, txHash common.Hash) error
	ValidateRevealVote(chainId string, from common.Address, pubkey string, depositAmount *big.Int, salt string) error
	RevealVote(ep *epoch.Epoch, from common.Address, pubkey string, depositAmount *big.Int, salt string, txHash common.Hash) error

	GetTxFromMainChain(txHash common.Hash) *types.Transaction
	GetTxFromChildChain(txHash common.Hash, chainId string) *types.Transaction
	VerifyChildChainProofData(bs []byte) error
	SaveChildChainProofDataToMainChain(bs []byte) error

	// these should operate on the main chain db
	MarkFromChildChainTx(from common.Address, chainId string, txHash common.Hash, used bool) error
	ValidateFromChildChainTx(from common.Address, chainId string, txHash common.Hash) CrossChainTxState
}

type EtdValidateCb func(tx *types.Transaction, state *state.StateDB, cch CrossChainHelper) error
type EtdApplyCb func(tx *types.Transaction, state *state.StateDB, ops *types.PendingOps, cch CrossChainHelper) error
type EtdInsertBlockCb func(bc *BlockChain, block *types.Block)

var validateCbMap = make(map[pabi.FunctionType]EtdValidateCb)
var applyCbMap = make(map[pabi.FunctionType]EtdApplyCb)
var insertBlockCbMap = make(map[string]EtdInsertBlockCb)

func RegisterValidateCb(function pabi.FunctionType, validateCb EtdValidateCb) error {

	_, ok := validateCbMap[function]
	if ok {
		return errors.New("the name has registered in validateCbMap")
	}

	validateCbMap[function] = validateCb
	return nil
}

func GetValidateCb(function pabi.FunctionType) EtdValidateCb {

	cb, ok := validateCbMap[function]
	if ok {
		return cb
	}

	return nil
}

func RegisterApplyCb(function pabi.FunctionType, applyCb EtdApplyCb) error {

	_, ok := applyCbMap[function]
	if ok {
		return errors.New("the name has registered in applyCbMap")
	}

	applyCbMap[function] = applyCb

	return nil
}

func GetApplyCb(function pabi.FunctionType) EtdApplyCb {

	cb, ok := applyCbMap[function]
	if ok {
		return cb
	}

	return nil
}

func RegisterInsertBlockCb(name string, insertBlockCb EtdInsertBlockCb) error {

	_, ok := insertBlockCbMap[name]
	if ok {
		return errors.New("the name has registered in insertBlockCbMap")
	}

	insertBlockCbMap[name] = insertBlockCb

	return nil
}

func GetInsertBlockCb(name string) EtdInsertBlockCb {

	cb, ok := insertBlockCbMap[name]
	if ok {
		return cb
	}

	return nil
}

func GetInsertBlockCbMap() map[string]EtdInsertBlockCb {

	return insertBlockCbMap
}
