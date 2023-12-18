package userop

import (
	"github.com/blndgs/model"
)

func (op *UserOperation) HasIntent() bool {
	modelUserOp := model.UserOperation(*op)

	return modelUserOp.HasIntent()
}
