package bundler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/blndgs/model"
	"github.com/ethereum/go-ethereum/common"

	"github.com/stackup-wallet/stackup-bundler/pkg/userop"
)

type IntentOpsBatch map[string]*userop.UserOperation
type IntentsBatch map[string]*model.Intent
type OrigBatchIdx map[string]int

type EntryPointIntents struct {
	EntryPoint     common.Address
	Intents        IntentsBatch   // set of intents in the batch
	IntentsOps     IntentOpsBatch // subset of userOp intents out of Batch
	UserOpsOrigIdx OrigBatchIdx   // map of userOp hash to its index in the original batch
	OrigBatch      []*userop.UserOperation
	InvalidIntents uint
}

func NewEntryPointIntents(entryPoint common.Address, origBatch []*userop.UserOperation) *EntryPointIntents {
	return &EntryPointIntents{
		EntryPoint:     entryPoint,
		Intents:        make(IntentsBatch),
		IntentsOps:     make(IntentOpsBatch),
		UserOpsOrigIdx: make(OrigBatchIdx),
		OrigBatch:      origBatch,
	}
}

func sendToSolver(solverClient *http.Client, solverURL string, senderAddress string, intents []*model.Intent) ([]*model.Intent, error) {
	// Create a Body instance and populate it
	body := model.Body{
		Intents: intents,
	}

	// Marshal the Body instance into JSON
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	// Create a new HTTP POST request
	req, err := http.NewRequest(http.MethodPost, solverURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		// Log error and return
		log.Printf("Error creating request: %s", err.Error())
		return nil, err
	}

	// Set the content type to application/json
	req.Header.Set("Content-Type", "application/json")

	// Send the request
	resp, err := solverClient.Do(req)
	if err != nil {
		log.Printf("Error occurred sending request. Error: %s", err.Error())
		return nil, err
	}
	defer resp.Body.Close()

	// Decode the returned intents back into the same slice of intents
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		log.Printf("Error decoding response: %s", err.Error())
		return nil, err
	}

	return body.Intents, nil
}

func (i *Bundler) solveIntents(intentsBatch *EntryPointIntents) {
	if len(intentsBatch.Intents) == 0 {
		return
	}

	l := i.logger.WithName("solveIntents")
	intents := make([]*model.Intent, len(intentsBatch.Intents))
	j := 0
	for _, itt := range intentsBatch.Intents {
		itt.Status = model.SentToSolver
		intents[j] = itt
		j++
	}

	// TODO: Add Backoff logic
	var err error
	intents, err = sendToSolver(i.solverClient, i.solverURL, intents[0].Sender, intents)
	if err != nil {
		l.WithValues("number_intents", len(intents)).
			Error(err, "failed to send intents to solver")
		return
	}

	for _, intent := range intents {
		switch intent.Status {
		case model.Solved:
			// Set the solution back to the original userOp
			intentsBatch.OrigBatch[intentsBatch.UserOpsOrigIdx[intent.Hash]].CallData = []byte(intent.CallData)
		case model.Unsolved:
			// will be retried till expired
			intentsBatch.Intents[intent.Hash].Status = model.Unsolved
		case model.Expired, model.Invalid:
			delete(intentsBatch.Intents, intent.Hash)
			delete(intentsBatch.IntentsOps, intent.Hash)
			l.WithValues("intent_hash", intent.Hash,
				"intent_status", intent.Status).
				Info("cannot process further")
		default:
			l.WithValues("intent_hash", intent.Hash,
				"intent_status", intent.Status).
				Error(fmt.Errorf("unknown intent status"), "unknown returned solver status")
		}
	}
}

func (i *Bundler) identifyIntents(entryPoint common.Address, batch []*userop.UserOperation) *EntryPointIntents {
	l := i.logger.WithName("identifyIntents").V(1)
	intentsBatch := NewEntryPointIntents(entryPoint, batch)

	for idx, userOp := range batch {
		opHash := userOp.GetUserOpHash(entryPoint, i.chainID).String()
		var intent model.Intent
		if userOp.IsIntent() {
			userOp.RemoveIntentPrefix()
			if err := json.Unmarshal(userOp.CallData, &intent); err != nil {
				l.WithValues(
					"userop_hash", opHash,
					"userop_nonce", userOp.Nonce,
					"userop_sender", userOp.Sender.String(),
					"is_intent", userOp.IsIntent,
					"call_data", userOp.CallData).
					Error(err, "failed to unmarshal intent")
				intentsBatch.InvalidIntents++
				continue
			}

			// Save the identified intent
			intentsBatch.IntentsOps[opHash] = userOp

			// Set the intent hash to userOp's
			intent.Hash = opHash
			if intent.CreatedAt == 0 {
				intent.CreatedAt = time.Now().Unix()
			}
			intentsBatch.Intents[opHash] = &intent

			// Save the index of the userOp in the original batch
			intentsBatch.UserOpsOrigIdx[opHash] = idx
		}
	}
	if len(intentsBatch.Intents) > 0 {
		i.solveIntents(intentsBatch)
	}

	return intentsBatch
}

func (i *Bundler) PreProcessIntents(entryPoint common.Address, userOpsBatch []*userop.UserOperation) *EntryPointIntents {
	intentsBatch := i.identifyIntents(entryPoint, userOpsBatch)
	if len(intentsBatch.Intents) == 0 {
		return intentsBatch
	}

	i.solveIntents(intentsBatch)

	return intentsBatch
}
