package main

import (
	"bytes"
	"fmt"
	"math/big"
	"net/http"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/goccy/go-json"
	"github.com/spf13/viper"

	"github.com/stackup-wallet/stackup-bundler/pkg/signer"
	"github.com/stackup-wallet/stackup-bundler/pkg/userop"
)

type JsonRpcRequest struct {
	Jsonrpc string        `json:"jsonrpc"`
	Id      int           `json:"id"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
}

func main() {

	viper.SetConfigName(".env")
	viper.SetConfigType("env")
	viper.AddConfigPath(".")
	if err := viper.ReadInConfig(); err != nil {
		panic(fmt.Errorf("fatal error config file: %w", err))
	}

	s, err := signer.New(viper.GetString("erc4337_bundler_private_key"))
	if err != nil {
		panic(fmt.Errorf("fatal signer error: %w", err))
	}
	fmt.Printf("Public key: %s\n", hexutil.Encode(crypto.FromECDSAPub(s.PublicKey))[4:])
	fmt.Printf("Address: %s\n", s.Address)

	sender := common.HexToAddress("0x3068c2408c01bECde4BcCB9f246b56651BE1d12D")
	nonce := big.NewInt(11)
	// initCode := hex.EncodeToString([]byte{})
	callData := `{"sender":"0x0A7199a96fdf0252E09F76545c1eF2be3692F46b","kind":"swap","hash":"","sellToken":"TokenA","buyToken":"TokenB","sellAmount":10,"buyAmount":5,"partiallyFillable":false,"status":"Received","createdAt":0,"expirationAt":0}`
	callGasLimit := big.NewInt(12052)
	verificationGasLimit := big.NewInt(58592)
	preVerificationGas := big.NewInt(48000)
	maxFeePerGas := big.NewInt(0xac97bb286)
	maxPriorityFeePerGas := big.NewInt(0xac97bb264)
	// paymasterAndData := hex.EncodeToString([]byte{})

	// Placeholder for signature
	// signature := hex.EncodeToString([]byte{})

	userOp := userop.UserOperation{
		Sender:               sender,
		Nonce:                nonce,
		InitCode:             []byte{},
		CallData:             []byte(callData),
		CallGasLimit:         callGasLimit,
		VerificationGasLimit: verificationGasLimit,
		PreVerificationGas:   preVerificationGas,
		MaxFeePerGas:         maxFeePerGas,
		MaxPriorityFeePerGas: maxPriorityFeePerGas,
		PaymasterAndData:     []byte{},
		Signature:            []byte{},
	}

	const entrypointAddr = "0x5FF137D4b0FDCD49DcA30c7CF57E578a026d2789"
	request := JsonRpcRequest{
		Jsonrpc: "2.0",
		Id:      3,
		Method:  "eth_sendUserOperation",
		Params:  []interface{}{userOp, entrypointAddr},
	}

	requestBytes, err := json.Marshal(request)
	if err != nil {
		panic(err)
	}

	resp, err := http.Post("http://localhost:4337", "application/json", bytes.NewBuffer(requestBytes))
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	// Decode the response
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		panic(err)
	}

	// Print the response
	fmt.Println("Response from server:", result)
}
