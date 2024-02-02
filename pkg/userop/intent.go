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

	status, err := modelUserOp.Validate()
	if err != nil || status != model.UnsolvedUserOp {
		return false
	}

	return true
}

func (op *UserOperation) IsSolvedIntent() bool {
	modelUserOp := model.UserOperation(*op)

	status, err := modelUserOp.Validate()
	if err != nil || status != model.SolvedUserOp {
		return false
	}

	return true
}
