package jsonrpc

import (
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/gin-gonic/gin"
)

type rpcBlock struct {
	Hash         common.Hash         `json:"hash"`
	Transactions []rpcTransaction    `json:"transactions"`
	UncleHashes  []common.Hash       `json:"uncles"`
	Withdrawals  []*types.Withdrawal `json:"withdrawals,omitempty"`
}

type rpcTransaction struct {
	tx *types.Transaction
	txExtraInfo
}

type txExtraInfo struct {
	BlockNumber *string         `json:"blockNumber,omitempty"`
	BlockHash   *common.Hash    `json:"blockHash,omitempty"`
	From        *common.Address `json:"from,omitempty"`
}

func (tx *rpcTransaction) UnmarshalJSON(msg []byte) error {
	if err := json.Unmarshal(msg, &tx.tx); err != nil {
		return err
	}
	return json.Unmarshal(msg, &tx.txExtraInfo)
}

func getBlock(c *gin.Context, method string, rpcClient *rpc.Client, requestData map[string]any) {
	var raw json.RawMessage

	params, ok := requestData["params"].([]interface{})
	if !ok {
		jsonrpcError(c, -32602, "Invalid params format", "Expected a slice of parameters", nil)
		return
	}

	err := rpcClient.CallContext(c, &raw, method, params...)
	if err != nil {
		jsonrpcError(c, -32603, "Internal error", err.Error(), nil)
		return
	}

	// Decode header and transactions.
	var head *types.Header
	if err := json.Unmarshal(raw, &head); err != nil {
		return
	}
	// When the block is not found, the API returns JSON null.
	if head == nil {
		return
	}

	var body rpcBlock
	if err := json.Unmarshal(raw, &body); err != nil {
		return
	}
	// Quick-verify transaction and uncle lists. This mostly helps with debugging the server.
	if head.UncleHash == types.EmptyUncleHash && len(body.UncleHashes) > 0 {
		return
	}
	if head.UncleHash != types.EmptyUncleHash && len(body.UncleHashes) == 0 {
		return
	}
	if head.TxHash == types.EmptyTxsHash && len(body.Transactions) > 0 {
		return
	}
	if head.TxHash != types.EmptyTxsHash && len(body.Transactions) == 0 {
		return
	}
	// Load uncles because they are not included in the block response.
	// var uncles []*types.Header
	// if len(body.UncleHashes) > 0 {
	// 	uncles = make([]*types.Header, len(body.UncleHashes))
	// 	reqs := make([]rpc.BatchElem, len(body.UncleHashes))
	// 	for i := range reqs {
	// 		reqs[i] = rpc.BatchElem{
	// 			Method: "eth_getUncleByBlockHashAndIndex",
	// 			Args:   []interface{}{body.Hash, hexutil.EncodeUint64(uint64(i))},
	// 			Result: &uncles[i],
	// 		}
	// 	}
	// 	if err := rpcClient.BatchCallContext(c, reqs); err != nil {
	// 		return
	// 	}
	// 	for i := range reqs {
	// 		if reqs[i].Error != nil {
	// 			return
	// 		}
	// 		if uncles[i] == nil {
	// 			return
	// 		}
	// 	}
	// }

	// Fill the sender cache of transactions in the block.
	txs := make([]*types.Transaction, len(body.Transactions))
	for i, tx := range body.Transactions {
		if tx.From != nil {
			setSenderFromServer(tx.tx, *tx.From, body.Hash)
		}
		txs[i] = tx.tx
	}

	emptyUncles := make([]*types.Header, 0)
	retBlock := types.NewBlockWithHeader(head).WithBody(txs, emptyUncles).WithWithdrawals(body.Withdrawals)

	c.JSON(http.StatusOK, gin.H{
		"result":  retBlock,
		"jsonrpc": "2.0",
		"id":      requestData["id"],
	})
	return
}

// senderFromServer is a types.Signer that remembers the sender address returned by the RPC
// server. It is stored in the transaction's sender address cache to avoid an additional
// request in TransactionSender.
type senderFromServer struct {
	addr      common.Address
	blockhash common.Hash
}

func (s *senderFromServer) Equal(other types.Signer) bool {
	os, ok := other.(*senderFromServer)
	return ok && os.blockhash == s.blockhash
}

func setSenderFromServer(tx *types.Transaction, addr common.Address, block common.Hash) {
	// Use types.Sender for side-effect to store our signer into the cache.
	types.Sender(&senderFromServer{addr, block}, tx)
}

func (s *senderFromServer) Sender(tx *types.Transaction) (common.Address, error) {
	if s.addr == (common.Address{}) {
		return common.Address{}, fmt.Errorf("sender not cached")
	}
	return s.addr, nil
}

func (s *senderFromServer) ChainID() *big.Int {
	panic("can't sign with senderFromServer")
}
func (s *senderFromServer) Hash(tx *types.Transaction) common.Hash {
	panic("can't sign with senderFromServer")
}
func (s *senderFromServer) SignatureValues(tx *types.Transaction, sig []byte) (R, S, V *big.Int, err error) {
	panic("can't sign with senderFromServer")
}
