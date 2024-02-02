package bundler

import (
	"encoding/json"

	"github.com/stackup-wallet/stackup-bundler/pkg/modules"
	"github.com/stackup-wallet/stackup-bundler/pkg/userop"
)

func adjustBatchSize(max int, batch []*userop.UserOperation) []*userop.UserOperation {
	if len(batch) > max && max > 0 {
		return batch[:max]
	}
	return batch
}

func PrintCtx(ctx *modules.UserOpHandlerCtx) {
	println("UserOpHandlerCtx")
	PrintUserOp(ctx.UserOp)
	penOps := ctx.GetPendingOps()
	println("penOps:", penOps)
	for _, op := range penOps {
		PrintUserOp(op)
	}
	println("ChainID:", ctx.ChainID)
	println("EntryPoint:", ctx.EntryPoint.String())
	println("Deposits:")
	deps := ctx.GetDeposits()
	for _, dep := range deps {
		println("dep:", dep.Deposit.String())
		println("staked:", dep.Staked)
		println("stake:", dep.Stake.String())
	}
}

func PrintUserOp(op *userop.UserOperation) {
	opJSON, err := json.Marshal(op)
	if err != nil {
		println("userOp JSON marshalling err:", err)
	}

	println("opJSON:", string(opJSON))
}
