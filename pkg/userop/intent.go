package userop

import "github.com/goccy/go-json"

// HasIntent checks if the userOp's `Calldata` is an intent userOp by checking
// whether it contains valid JSON.
func (op *UserOperation) HasIntent() bool {
	var js json.RawMessage
	return json.Unmarshal(op.CallData, &js) == nil
}
