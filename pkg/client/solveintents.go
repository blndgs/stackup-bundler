package client

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/blndgs/model"
	"github.com/ethereum/go-ethereum/common"
	"github.com/goccy/go-json"

	"github.com/stackup-wallet/stackup-bundler/pkg/userop"
)

type EntryPointsIntents map[common.Address]*EntryPointIntents

type EntryPointIntents struct {
	EntryPoint      common.Address
	NewIntent       *model.Intent
	NewIntentUserOp *userop.UserOperation // the userOp intent or nil
	Unsolved        *Queue[*model.Intent]
	Buffer          map[string]*userop.UserOperation // buffer for intent userOps to be sent to Solver
	InvalidIntents  uint
}

func NewEntryPointIntent(entryPoint common.Address, origOp *userop.UserOperation) *EntryPointIntents {
	const unsolvedCap = 5
	return &EntryPointIntents{
		EntryPoint:      entryPoint,
		NewIntentUserOp: origOp,
		Unsolved:        NewQueue[*model.Intent](unsolvedCap),
		Buffer:          make(map[string]*userop.UserOperation),
	}
}

func sendToSolver(solverClient *http.Client, solverURL string, intents []*model.Intent) ([]*model.Intent, error) {
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

func (i *Client) solveIntent(entrypointIntent *EntryPointIntents) {
	l := i.logger.WithName("solveIntents")
	if entrypointIntent.NewIntentUserOp == nil {
		l.Error(
			fmt.Errorf("entryPointEntries or NewIntentUserOp is nil"),
			"")
	}

	hash := entrypointIntent.NewIntentUserOp.GetUserOpHash(entrypointIntent.EntryPoint, i.chainID).String()

	intent := model.Intent{
		Hash:   hash,
		Status: model.SentToSolver,
	}

	// Add the new NewIntent first to solve
	entrypointIntent.Unsolved.EnqueueHead(hash, &intent)
	entrypointIntent.Unsolved.Range(func(index int, value *model.Intent) {
		value.Status = model.SentToSolver
	})

	// TODO: Add Backoff logic
	var err error
	intents, err := sendToSolver(i.solverClient, i.solverURL, entrypointIntent.Unsolved.ToSlice())
	if err != nil {
		l.WithValues("number_intents", len(intents)).
			Error(err, "failed to send intents to solver")
	}

	entrypointIntent.Unsolved.Reset(uint(len(intents)))

	for _, intent := range intents {
		if intent.ExpirationAt < time.Now().Unix() {
			// expired, log & drop
			l.WithValues("intent_hash", intent.Hash,
				"intent_status", intent.Status).
				Info("dropping expired intent")
			continue
		}
		switch intent.Status {
		case model.Solved:
			// Set the solution back to the original userOp
			entrypointIntent.Unsolved.EnqueueTail(intent.Hash, intent)
		case model.Unsolved:
			// will be retried till expired
			entrypointIntent.Unsolved.EnqueueHead(intent.Hash, intent)
		default:
			// invalid or expired
			l.WithValues("intent_hash", intent.Hash,
				"intent_status", intent.Status).
				Info("dropping intent")
		}
	}
}

func (i *Client) identifyIntent(entrypointIntent *EntryPointIntents, userOp *userop.UserOperation) bool {
	l := i.logger.WithName("identifyIntents")
	opHash := userOp.GetUserOpHash(entrypointIntent.EntryPoint, i.chainID).String()
	if !userOp.HasIntent() {
		i.logger.WithValues("userop_hash", opHash,
			"userop_nonce", userOp.Nonce,
			"userop_sender", userOp.Sender.String(),
			"userop_call_data", string(userOp.CallData)).
			Info("userOp is not an intent")

		return false
	}

	var intent model.Intent
	if err := json.Unmarshal(userOp.CallData, &intent); err != nil {
		l.WithValues(
			"userop_hash", opHash,
			"userop_nonce", userOp.Nonce,
			"userop_sender", userOp.Sender.String(),
			"is_intent", userOp.HasIntent(),
			"call_data", userOp.CallData).
			Error(err, "failed to unmarshal intent")
		entrypointIntent.InvalidIntents++

		return false
	}

	// Save the identified intent
	entrypointIntent.NewIntentUserOp = userOp
	entrypointIntent.Buffer[opHash] = userOp
	entrypointIntent.NewIntent = &intent
	entrypointIntent.NewIntent.Hash = opHash
	intent.Status = model.Received
	entrypointIntent.Unsolved.EnqueueHead(opHash, &intent)

	// Set the intent hash to userOp's
	intent.Hash = opHash
	if intent.CreatedAt == 0 {
		intent.CreatedAt = time.Now().Unix()
	}
	if intent.ExpirationAt == 0 {
		intent.ExpirationAt = time.Unix(intent.CreatedAt, 0).Add(time.Duration(100 * time.Second)).Unix()
	}

	return true
}

func (i *Client) processIntent(entrypoint common.Address, userOp *userop.UserOperation) {
	if userOp == nil {
		i.logger.Error(fmt.Errorf("userOp is nil"), "userOp is nil")
	}
	if !userOp.HasIntent() {
		i.logger.WithValues("userop_hash", userOp.GetUserOpHash(entrypoint, i.chainID).String(),
			"userop_nonce", userOp.Nonce,
			"userop_sender", userOp.Sender.String(),
			"userop_call_data", string(userOp.CallData)).
			Info("userOp is not an intent")

		return
	}

	if i.entryPointsIntents[entrypoint] == nil {
		i.entryPointsIntents[entrypoint] = NewEntryPointIntent(entrypoint, userOp)

		// TODO: Add scheduling logic for unsolved intents
	}

	entrypointIntent := i.entryPointsIntents[entrypoint]

	if i.identifyIntent(entrypointIntent, userOp) {
		i.solveIntent(entrypointIntent)
	}
}
