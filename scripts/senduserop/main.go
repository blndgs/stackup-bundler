package main

import (
	"bytes"
	"crypto/ecdsa"
	"fmt"
	"log"
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

const entrypointAddr = "0x5FF137D4b0FDCD49DcA30c7CF57E578a026d2789"

func main() {

	viper.SetConfigName(".env")
	viper.SetConfigType("env")
	viper.AddConfigPath(".")
	if err := viper.ReadInConfig(); err != nil {
		panic(fmt.Errorf("fatal error config file: %w", err))
	}

	prvKeyHex := viper.GetString("erc4337_bundler_private_key")
	s, err := signer.New(prvKeyHex)
	if err != nil {
		panic(fmt.Errorf("fatal signer error: %w", err))
	}
	fmt.Printf("Private key: %s\n", hexutil.Encode(crypto.FromECDSA(s.PrivateKey)))
	fmt.Printf("Public key: %s\n", hexutil.Encode(crypto.FromECDSAPub(s.PublicKey))[4:])
	fmt.Printf("Address: %s\n", s.Address)

	verifySignedMessage(s.PrivateKey)

	sender := common.HexToAddress("0x3068c2408c01bECde4BcCB9f246b56651BE1d12D")
	nonce := big.NewInt(0x10)
	// initCode := hex.EncodeToString([]byte{})
	callData := `{"sender":"0x0A7199a96fdf0252E09F76545c1eF2be3692F46b","kind":"swap","hash":"","sellToken":"TokenA","buyToken":"TokenB","sellAmount":10,"buyAmount":5,"partiallyFillable":false,"status":"Received","createdAt":0,"expirationAt":0}`
	cdHex := hexutil.Encode([]byte(callData))

	println(callData, cdHex)

	callGasLimit := big.NewInt(0x2f44) // error if below 12100
	verificationGasLimit := big.NewInt(0xe4e0)
	preVerificationGas := big.NewInt(0xbb7c)
	maxFeePerGas := big.NewInt(0x12183576da)
	maxPriorityFeePerGas := big.NewInt(0x12183576ba)
	// paymasterAndData := hex.EncodeToString([]byte{})

	// Placeholder for signature

	userOp := &userop.UserOperation{
		Sender:               sender,
		Nonce:                nonce,
		InitCode:             []byte{},
		CallData:             []byte{}, // []byte(callData),
		CallGasLimit:         callGasLimit,
		VerificationGasLimit: verificationGasLimit,
		PreVerificationGas:   preVerificationGas,
		MaxFeePerGas:         maxFeePerGas,
		MaxPriorityFeePerGas: maxPriorityFeePerGas,
		PaymasterAndData:     []byte{},
	}

	privateKey, err := crypto.HexToECDSA(prvKeyHex)
	if err != nil {
		panic(err)
	}

	// signature := getVerifiedSignature(&userOp, privateKey)
	userOp.Signature = getVerifiedSignature(userOp, privateKey)

	// Verify the signature
	if verifySignature(userOp, &privateKey.PublicKey) {
		println("Signature is valid")
	} else {
		println("Signature is invalid")
	}

	jsonStr, err := userOp.MarshalJSON()
	if err != nil {
		panic(err)
	}
	println(string(jsonStr))

	request := JsonRpcRequest{
		Jsonrpc: "2.0",
		Id:      45,
		Method:  "eth_sendUserOperation",
		Params:  []interface{}{userOp, entrypointAddr},
	}

	requestBytes, err := json.Marshal(request)
	if err != nil {
		panic(err)
	}
	println(string(requestBytes))

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
	println("Response from server:", result)
}

func getVerifiedSignature(userOp *userop.UserOperation, privateKey *ecdsa.PrivateKey) []byte {
	userOpHash := userOp.GetUserOpHash(common.HexToAddress(entrypointAddr), big.NewInt(80001)).Bytes()

	prefixedHash := crypto.Keccak256Hash(
		[]byte(fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", len(userOpHash), userOpHash)),
	)

	signature, err := crypto.Sign(prefixedHash.Bytes(), privateKey)
	if err != nil {
		panic(err)
	}

	// Normalize S value for Ethereum
	sValue := big.NewInt(0).SetBytes(signature[32:64])
	secp256k1N := crypto.S256().Params().N
	if sValue.Cmp(new(big.Int).Rsh(secp256k1N, 1)) > 0 {
		sValue.Sub(secp256k1N, sValue)
		copy(signature[32:64], sValue.Bytes())
	}

	return signature
}

func verifySignature(userOp *userop.UserOperation, publicKey *ecdsa.PublicKey) bool {
	userOpHash := userOp.GetUserOpHash(common.HexToAddress(entrypointAddr), big.NewInt(80001)).Bytes()

	prefixedHash := crypto.Keccak256Hash(
		[]byte(fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", len(userOpHash), userOpHash)),
	)

	signature := userOp.Signature // Already in RSV format

	recoveredPubKey, err := crypto.SigToPub(prefixedHash.Bytes(), signature)
	if err != nil {
		fmt.Printf("Failed to recover public key: %v\n", err)
		return false
	}

	recoveredAddress := crypto.PubkeyToAddress(*recoveredPubKey)
	expectedAddress := crypto.PubkeyToAddress(*publicKey)

	return recoveredAddress == expectedAddress
}

func verifySignedMessage(privateKey *ecdsa.PrivateKey /*publicKey *ecdsa.PublicKey, address common.Address*/) {
	publicKeyECDSA, ok := privateKey.Public().(*ecdsa.PublicKey)
	if !ok {
		log.Fatal("error casting public key to ECDSA")
	}
	address := crypto.PubkeyToAddress(*publicKeyECDSA)

	// Sample message
	message := "Hello, Ethereum!"
	prefixedHash := crypto.Keccak256Hash([]byte(fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", len(message), message)))

	// Sign the message
	signature, err := crypto.Sign(prefixedHash.Bytes(), privateKey)
	if err != nil {
		log.Fatal(err)
	}

	// Normalize S value
	sValue := big.NewInt(0).SetBytes(signature[32:64])
	// Curve order for secp256k1
	secp256k1N := crypto.S256().Params().N
	if sValue.Cmp(new(big.Int).Rsh(secp256k1N, 1)) > 0 {
		sValue.Sub(secp256k1N, sValue)
		copy(signature[32:64], sValue.Bytes())
	}

	// Recover the public key without adjusting V
	recoveredPubKey, err := crypto.SigToPub(prefixedHash.Bytes(), signature)
	if err != nil {
		log.Fatal(err)
	}
	recoveredAddress := crypto.PubkeyToAddress(*recoveredPubKey)

	fmt.Printf("Original Address: %s\n", address.Hex())
	fmt.Printf("Recovered Address: %s\n", recoveredAddress.Hex())

	// Check if the recovered address matches the original address
	if address.Hex() == recoveredAddress.Hex() {
		fmt.Println("Signature valid, recovered address matches the original address")
	} else {
		fmt.Println("Invalid signature, recovered address does not match")
	}
}
