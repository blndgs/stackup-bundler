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
	_, hasCalldataIntent := model.ExtractJSONFromField(string(modelUserOp.CallData))

	return hasCalldataIntent
}

func (op *UserOperation) IsIntentExecutable() bool {
	modelUserOp := model.UserOperation(*op)

	var hasSigIntent bool
	if len(modelUserOp.Signature) > model.SignatureLength {
		_, hasSigIntent = model.ExtractJSONFromField(string(modelUserOp.Signature[model.SignatureLength:]))
	}

	return hasSigIntent
}
