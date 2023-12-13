package solution

import (
	"bytes"
	"math/big"
	"net/http"
	"time"
	"unsafe"

	"github.com/blndgs/model"
	"github.com/ethereum/go-ethereum/common"
	"github.com/goccy/go-json"

	"github.com/stackup-wallet/stackup-bundler/pkg/modules"
	"github.com/stackup-wallet/stackup-bundler/pkg/userop"
)

type EntryPointsIntents map[common.Address]*EntryPointIntents

type EntryPointIntents struct {
	SolverURL    string
	SolverClient *http.Client
	Buffer       map[string]int // buffer for Hash to index of userOp in batch
	Ops          []*userop.UserOperation
}

func New(solverURL string) *EntryPointIntents {
	const httpClientTimeout = 100 * time.Second

	return &EntryPointIntents{
		SolverURL:    solverURL,
		SolverClient: &http.Client{Timeout: httpClientTimeout},
		Buffer:       make(map[string]int),
	}
}

// bufferSentUserOp caches the index of the userOp in the batch
func (ei *EntryPointIntents) bufferSentUserOp(bodyOfUserOps model.BodyOfUserOps) {
	// Cache the index of the userOp in the batch
	for idx, opExt := range bodyOfUserOps.UserOpsExt {
		ei.Buffer[opExt.OriginalHashValue] = idx
	}
}

// getExtSlice returns a slice of UserOperationExt with cached Hashes and ProcessingStatus set to Received.
func getExtSlice(entrypoint common.Address, chainID *big.Int, userOps []*model.UserOperation) []model.UserOperationExt {
	userOpsExt := make([]model.UserOperationExt, len(userOps))
	for idx, op := range userOps {
		userOpsExt[idx].ProcessingStatus = model.Received

		// Cache hash before it changes
		userOpsExt[idx].OriginalHashValue = op.GetUserOpHash(entrypoint, chainID).String()
	}

	return userOpsExt
}

// SolveIntents returns a BatchHandlerFunc that will send the batch of UserOperations to the Solver and
// mark the UserOperations that have been expired been solved to be sent on chain.
func (ei *EntryPointIntents) SolveIntents() modules.BatchHandlerFunc {
	return func(ctx *modules.BatchHandlerCtx) error {
		// cast the received userOp batch to a slice of model.UserOperation
		modelUserOps := *(*[]*model.UserOperation)(unsafe.Pointer(&ctx.Batch))

		// Rest of the sendToSolver logic
		body := model.BodyOfUserOps{
			UserOps:    modelUserOps,
			UserOpsExt: getExtSlice(ctx.EntryPoint, ctx.ChainID, modelUserOps),
		}

		// batch Ops hashes have been cached before calling this function
		ei.bufferSentUserOp(body)

		jsonBody, err := json.Marshal(body)
		if err != nil {
			return err
		}

		req, err := http.NewRequest(http.MethodPost, ei.SolverURL, bytes.NewBuffer(jsonBody))
		if err != nil {
			return err
		}

		req.Header.Set("Content-Type", "application/json")

		resp, err := ei.SolverClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			return err
		}

		// Mark the userOps that have been expired or invalid to be removed from the batch
		for idx, opExt := range body.UserOpsExt {
			if opExt.ProcessingStatus == model.Expired || opExt.ProcessingStatus == model.Invalid {
				batchIndex := ei.Buffer[body.UserOpsExt[idx].OriginalHashValue]
				ctx.MarkOpIndexForRemoval(batchIndex)

				// Remove the userOp from the buffer
				delete(ei.Buffer, body.UserOpsExt[idx].OriginalHashValue)
			}
			switch opExt.ProcessingStatus {
			case model.Expired, model.Invalid:
				// dropping further processing of the userOp
				batchIndex := ei.Buffer[body.UserOpsExt[idx].OriginalHashValue]
				ctx.MarkOpIndexForRemoval(batchIndex)
				delete(ei.Buffer, body.UserOpsExt[idx].OriginalHashValue)
			case model.Solved:
				// Remove the Solved userOp from the Solver buffer
				// Remain in the mempool until the userOp is sent on chain
				delete(ei.Buffer, body.UserOpsExt[idx].OriginalHashValue)
			}
		}

		return nil
	}
}
