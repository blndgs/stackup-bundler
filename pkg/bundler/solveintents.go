package bundler

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"time"

	intentsdt "github.com/blndgs/intents"

	"github.com/stackup-wallet/stackup-bundler/pkg/userop"
)

func sendToSolver(solverClient *http.Client, intents []*intentsdt.Intent) ([]*intentsdt.Intent, error) {
	jsonBody, err := json.Marshal(intents)
	if err != nil {
		return nil, err
	}

	// Create a new HTTP POST request
	req, err := http.NewRequest(http.MethodPost, "http://localhost:7322", bytes.NewBuffer(jsonBody))
	if err != nil {
		// TODO log error
		return nil, err
	}

	// Set the content type to application/json
	req.Header.Set("Content-Type", "application/json")

	// Send the request
	resp, err := solverClient.Do(req)
	if err != nil {
		log.Fatalf("Error occurred sending request. Error: %s", err.Error())
	}
	defer resp.Body.Close()

	// Decode the response body into intents
	err = json.NewDecoder(resp.Body).Decode(&intents)

	return intents, nil
}

func solveIntents(solverClient *http.Client, intentOps []*userop.UserOperation) ([]*userop.UserOperation, error) {
	intents := make([]*intentsdt.Intent, len(intentOps))
	var (
		errorsCnt int
		lastErr   error
		err       error
	)
	mapIntentOps := make(map[string]*userop.UserOperation)
	for i, op := range intentOps {
		op.RemoveIntentPrefix()
		err := json.Unmarshal(op.CallData, &intents[i])
		if err != nil {
			// TODO: log and continue
			errorsCnt++
			lastErr = err
			continue
		}
		mapIntentOps[intents[i].Hash] = op
		intents[i].Status = intentsdt.SentToSolver
		if intents[i].CreatedAt == 0 {
			intents[i].CreatedAt = uint64(time.Now().Unix())
		}
	}

	intents, err = sendToSolver(solverClient, intents)
	if err != nil {
		return nil, err
	}

	for _, intent := range intents {
		if intent.Status == intentsdt.Solved {
			mapIntentOps[intent.Hash].CallData = []byte(intent.CallData)
		}
		// TODO: handle unsolved, invalid intents, log etc.
	}

	return intentOps, lastErr
}
