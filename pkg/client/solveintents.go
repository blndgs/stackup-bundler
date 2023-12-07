package client

import (
	"bytes"
	"fmt"
	"net/http"
	"time"

	"github.com/blndgs/model"
	"github.com/ethereum/go-ethereum/common"
	"github.com/go-logr/logr"
	"github.com/goccy/go-json"

	"github.com/stackup-wallet/stackup-bundler/pkg/userop"
)

type EntryPointsIntents map[common.Address]*EntryPointIntents

type EntryPointIntents struct {
	EntryPoint     common.Address
	Unsolved       *Queue[*model.Intent]
	Buffer         map[string]*userop.UserOperation // buffer for intent userOps to be sent to Solver
	InvalidIntents uint
}

func NewEntryPointIntent(entryPoint common.Address) *EntryPointIntents {
	const unsolvedCap = 5
	return &EntryPointIntents{
		EntryPoint: entryPoint,
		Unsolved:   NewQueue[*model.Intent](unsolvedCap),
		Buffer:     make(map[string]*userop.UserOperation),
	}
}

func sendToSolver(log logr.Logger, unsolvedQ *Queue[*model.Intent], solvedOps chan *userop.UserOperation,
	epIntents *EntryPointIntents, solverClient *http.Client, solverURL string) func() {
	return func() {
		l := log.WithName("sendToSolver")
		// Get the unsolved intents from the queue
		intents := unsolvedQ.ToSlice()

		// If there are no intents, return
		if len(intents) == 0 {
			return
		}

		epIntents.Unsolved.Reset(len(intents))

		// Rest of the sendToSolver logic
		body := model.Body{
			Intents: intents,
		}
		jsonBody, err := json.Marshal(body)
		if err != nil {
			l.WithValues("number_intents", len(intents)).
				Error(err, "failed to marshal intents")
			return
		}

		req, err := http.NewRequest(http.MethodPost, solverURL, bytes.NewBuffer(jsonBody))
		if err != nil {
			l.WithValues("number_intents", len(intents)).
				Error(err, "failed to create request")
			return
		}

		req.Header.Set("Content-Type", "application/json")

		resp, err := solverClient.Do(req)
		if err != nil {
			l.WithValues("number_intents", len(intents)).
				Error(err, "failed to send request")
			return
		}
		defer resp.Body.Close()

		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			l.WithValues("number_intents", len(intents)).
				Error(err, "failed to decode response")
			return
		}

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
				// Set the solution to be processed by the bundler client
				solvedUserOp := epIntents.Buffer[intent.Hash]
				solvedUserOp.CallData = []byte(intent.CallData)
				solvedOps <- solvedUserOp
				delete(epIntents.Buffer, intent.Hash)
			case model.Unsolved:
				// will be retried till expired
				epIntents.Unsolved.EnqueueHead(intent.Hash, intent)
			default:
				// invalid or expired
				l.WithValues("intent_hash", intent.Hash,
					"intent_status", intent.Status).
					Info("dropping intent")
			}
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
	entrypointIntent.Buffer[opHash] = userOp
	intent.Hash = opHash
	intent.Status = model.Received

	// Set the intent hash to userOp's
	intent.Hash = opHash
	if intent.CreatedAt == 0 {
		intent.CreatedAt = time.Now().Unix()
	}
	if intent.ExpirationAt == 0 {
		// TODO: set intents expiration configurable
		const ttl = time.Duration(100 * time.Second)
		intent.ExpirationAt = time.Unix(intent.CreatedAt, 0).Add(ttl).Unix()
	}

	entrypointIntent.Unsolved.EnqueueHead(opHash, &intent)

	return true
}

// processIntent solves intents from new received Intent userOps
func (i *Client) processIntent(entrypoint common.Address, userOp *userop.UserOperation) {
	l := i.logger.WithName("processIntent")

	if userOp == nil {
		l.Error(fmt.Errorf("userOp is nil"), "userOp is nil")
	}
	if !userOp.HasIntent() {
		l.WithValues("userop_hash", userOp.GetUserOpHash(entrypoint, i.chainID).String(),
			"userop_nonce", userOp.Nonce,
			"userop_sender", userOp.Sender.String(),
			"userop_call_data", string(userOp.CallData)).
			Info("userOp is not an intent")

		return
	}

	if i.entryPointsIntents[entrypoint] == nil {
		ep := NewEntryPointIntent(entrypoint)
		i.entryPointsIntents[entrypoint] = ep
		scheduledFunc := sendToSolver(i.logger, ep.Unsolved, i.solvedOps, ep, i.solverClient, i.solverURL)

		// Start scheduling the sendToSolver function
		ep.Unsolved.SetTickerFunc(time.Second*1, scheduledFunc)
	}

	entrypointIntents := i.entryPointsIntents[entrypoint]

	i.identifyIntent(entrypointIntents, userOp)
}

// processIntentUserOps consumes solved Intent userOps
func (i *Client) processIntentUserOps(entrypoint common.Address) {
	l := i.logger.WithName("client.processIntentUserOps")

	for userOp := range i.solvedOps {

		println("A solved userOp: ", userOp, " popped")

		go func(entrypoint common.Address, userOp *userop.UserOperation) {

			println("Adding to mempool the solved userOp: ", string(userOp.CallData))

			hashOp, err := i.addToMemPool(entrypoint, userOp)
			if err != nil {
				l.WithValues("userop_hash", hashOp,
					"userop_nonce", userOp.Nonce,
					"userop_sender", userOp.Sender.String(),
					"userop_call_data", string(userOp.CallData),
					"entrypoint", entrypoint.String()).
					Error(err, "failed to add userOp to mempool")
			}
		}(entrypoint, userOp)
	}
}
