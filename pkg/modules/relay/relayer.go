// Package relay implements a module for private bundlers to send batches to the EntryPoint through regular
// EOA transactions.
package relay

import (
	"math/big"
	"time"

	"github.com/blndgs/model"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/go-logr/logr"

	"github.com/stackup-wallet/stackup-bundler/pkg/entrypoint/transaction"
	"github.com/stackup-wallet/stackup-bundler/pkg/modules"
	"github.com/stackup-wallet/stackup-bundler/pkg/signer"
	"github.com/stackup-wallet/stackup-bundler/pkg/userop"
)

// Relayer provides a module that can relay batches with a regular EOA. Relaying batches to the EntryPoint
// through a regular transaction comes with several important notes:
//
//   - The bundler will NOT be operating as a block builder.
//   - This opens the bundler up to frontrunning.
//
// This module only works in the case of a private mempool and will not work in the P2P case where ops are
// propagated through the network and it is impossible to prevent collisions from multiple bundlers trying to
// relay the same ops.
type Relayer struct {
	eoa         *signer.EOA
	eth         *ethclient.Client
	chainID     *big.Int
	beneficiary common.Address
	logger      logr.Logger
	waitTimeout time.Duration
}

// New initializes a new EOA relayer for sending batches to the EntryPoint.
func New(
	eoa *signer.EOA,
	eth *ethclient.Client,
	chainID *big.Int,
	beneficiary common.Address,
	l logr.Logger,
) *Relayer {
	return &Relayer{
		eoa:         eoa,
		eth:         eth,
		chainID:     chainID,
		beneficiary: beneficiary,
		logger:      l.WithName("relayer"),
		waitTimeout: DefaultWaitTimeout,
	}
}

// SetWaitTimeout sets the total time to wait for a transaction to be included. When a timeout is reached, the
// BatchHandler will throw an error if the transaction has not been included or has been included but with a
// failed status.
//
// The default value is 30 seconds. Setting the value to 0 will skip waiting for a transaction to be included.
func (r *Relayer) SetWaitTimeout(timeout time.Duration) {
	r.waitTimeout = timeout
}

// SendUserOperation returns a BatchHandler that is used by the Bundler to send batches in a regular EOA
// transaction.
func (r *Relayer) SendUserOperation() modules.BatchHandlerFunc {
	return func(ctx *modules.BatchHandlerCtx) error {
		// Filter out UserOperations on HasIntent() result
		nonIntentsBatch := make([]*userop.UserOperation, 0, len(ctx.Batch))
		intentsBatch := make([]*userop.UserOperation, 0, len(ctx.Batch))
		for _, userOp := range ctx.Batch {
			if userOp.IsSolvedIntent() {
				// Solved Intent UserOperations
				intentsBatch = append(intentsBatch, userOp)

			} else if !userOp.HasIntent() {
				// conventional UserOperation
				nonIntentsBatch = append(nonIntentsBatch, userOp)

			} else {
				// Do not send unsolved Intents to the EntryPoint
				r.logger.WithValues("userOp Hash", userOp.GetUserOpHash(ctx.EntryPoint, ctx.ChainID)).
					Info("unsolved intent not sent to entrypoint")
			}
		}

		// Only proceed if there are conventional UserOperations to process
		if len(nonIntentsBatch) > 0 {
			opts := r.getCallOptions(ctx, nonIntentsBatch)

			// Estimate gas for handleOps() and drop all userOps that cause unexpected reverts.
			estRev := []string{}
			for len(nonIntentsBatch) > 0 {
				est, revert, err := transaction.EstimateHandleOpsGas(&opts)

				if err != nil {
					return err
				} else if revert != nil {
					ctx.MarkOpIndexForRemoval(revert.OpIndex)
					estRev = append(estRev, revert.Reason)
				} else {
					opts.GasLimit = est
					break
				}
			}
			ctx.Data["relayer_est_revert_reasons"] = estRev

			// Call handleOps() with gas estimate. Any userOps that cause a revert at this stage will be
			// caught and dropped in the next iteration.
			if err := handleOps(ctx, opts); err != nil {
				return err
			}

			return nil
		} // end of sending conventional userOps

		if len(intentsBatch) > 0 {
			opts := r.getCallOptions(ctx, intentsBatch)
			println()
			for _, op := range intentsBatch {
				// cast to print it
				operation := model.UserOperation(*op)
				println(operation.String())
			}
			println()
			println("--> handleOps")

			if err := handleOps(ctx, opts); err != nil {
				// swallow error
				println(err.Error())
			}
		}

		return nil
	}
}

func handleOps(ctx *modules.BatchHandlerCtx, opts transaction.Opts) error {
	if txn, err := transaction.HandleOps(&opts); err != nil {
		return err
	} else {
		ctx.Data["txn_hash"] = txn.Hash().String()
	}

	return nil
}

func (r *Relayer) getCallOptions(ctx *modules.BatchHandlerCtx, intentsBatch []*userop.UserOperation) transaction.Opts {
	opts := transaction.Opts{
		EOA:         r.eoa,
		Eth:         r.eth,
		ChainID:     ctx.ChainID,
		EntryPoint:  ctx.EntryPoint,
		Batch:       intentsBatch,
		Beneficiary: r.beneficiary,
		BaseFee:     ctx.BaseFee,
		Tip:         ctx.Tip,
		GasPrice:    ctx.GasPrice,
		GasLimit:    0,
		WaitTimeout: r.waitTimeout,
	}
	return opts
}
