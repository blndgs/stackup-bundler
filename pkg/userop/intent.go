package userop

import (
	"github.com/blndgs/model"
)

func (op *UserOperation) HasIntent() bool {
	modelUserOp := model.UserOperation(*op)

	return modelUserOp.HasIntent()
}

func (op *UserOperation) IsUnsolvedIntent() bool {
	modelUserOp := model.UserOperation(*op)

	return modelUserOp.HasIntent() && !modelUserOp.HasEVMInstructions()
}

func (op *UserOperation) IsIntentExecutable() bool {
	modelUserOp := model.UserOperation(*op)

	return modelUserOp.HasIntent() && modelUserOp.HasEVMInstructions()
}
