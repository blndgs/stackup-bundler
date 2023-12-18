// Scripted test wallet functionality for submitting Intent userOps to the
// bundler.
// Sends semi-mocked userOps to the bundler with live nonce,
// chainID values. Support submitting userOps 0 gas for testing.
// UserOp signature verification is included.
package main

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"net/http"
	"os"

	"github.com/blndgs/model"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
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

const entrypointAddrV060 = "0x5FF137D4b0FDCD49DcA30c7CF57E578a026d2789"

func main() {
	nodeURL, eoaSigner := readConf()

	sender := common.HexToAddress("0x3068c2408c01bECde4BcCB9f246b56651BE1d12D")

	nonce, chainID, err := getNodeIDs(nodeURL, eoaSigner.Address)
	if err != nil {
		panic(err)
	}

	zeroGas := (len(os.Args) > 1 && (os.Args[1] == "zero" || os.Args[1] == "0")) || len(os.Args) == 1 // default choice
	unsignedUserOp := getMockUserOp(sender, nonce, zeroGas)

	userOp := getVerifiedSignedUserOp(unsignedUserOp, eoaSigner.PrivateKey, eoaSigner.PublicKey, chainID)

	sendUserOp(userOp, chainID)
}

// sendUserOp makes a UserOperation RPC request to the bundler.
func sendUserOp(userOp *userop.UserOperation, chainID *big.Int) {
	userOpHash := userOp.GetUserOpHash(common.HexToAddress(entrypointAddrV060), chainID).String()

	println("userOp (", userOpHash, ") ------------> bundler")
	op := model.UserOperation(*userOp)
	println(op.String())
	println()

	request := JsonRpcRequest{
		Jsonrpc: "2.0",
		Id:      45,
		Method:  "eth_sendUserOperation",
		Params:  []interface{}{userOp, entrypointAddrV060},
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
	println("Response from server:", result)
}

func getMockUserOp(sender common.Address, nonce *big.Int, zeroGas bool) *userop.UserOperation {
	intentJSON := `{"sender":"0x0A7199a96fdf0252E09F76545c1eF2be3692F46b","kind":"swap","hash":"","sellToken":"TokenA","buyToken":"TokenB","sellAmount":10,"buyAmount":5,"partiallyFillable":false,"status":"Received","createdAt":0,"expirationAt":0}`

	// Conditional gas values based on zeroGas flag
	var callGasLimit, verificationGasLimit, preVerificationGas, maxFeePerGas, maxPriorityFeePerGas *big.Int
	if zeroGas {
		callGasLimit = big.NewInt(0)
		verificationGasLimit = big.NewInt(0)
		preVerificationGas = big.NewInt(0)
		maxFeePerGas = big.NewInt(0)
		maxPriorityFeePerGas = big.NewInt(0)
	} else {
		callGasLimit = big.NewInt(0x2f44) // error if below 12100
		verificationGasLimit = big.NewInt(0xe4e0)
		preVerificationGas = big.NewInt(0xbb7c)
		maxFeePerGas = big.NewInt(0x12183576da)
		maxPriorityFeePerGas = big.NewInt(0x12183576ba)
	}

	return &userop.UserOperation{
		Sender:               sender,
		Nonce:                nonce,
		InitCode:             []byte{},
		CallData:             []byte(intentJSON),
		CallGasLimit:         callGasLimit,
		VerificationGasLimit: verificationGasLimit,
		PreVerificationGas:   preVerificationGas,
		MaxFeePerGas:         maxFeePerGas,
		MaxPriorityFeePerGas: maxPriorityFeePerGas,
		PaymasterAndData:     []byte{},
	}
}

func readConf() (string, *signer.EOA) {
	viper.SetConfigName(".env")
	viper.SetConfigType("env")
	viper.AddConfigPath(".")
	if err := viper.ReadInConfig(); err != nil {
		panic(fmt.Errorf("fatal error config file: %w", err))
	}

	nodeURL := viper.GetString("ERC4337_BUNDLER_ETH_CLIENT_URL")
	prvKeyHex := viper.GetString("erc4337_bundler_private_key")
	s, err := signer.New(prvKeyHex)
	if err != nil {
		panic(fmt.Errorf("fatal signer error: %w", err))
	}

	fmt.Printf("Private key: %s\n", hexutil.Encode(crypto.FromECDSA(s.PrivateKey)))
	fmt.Printf("Public key: %s\n", hexutil.Encode(crypto.FromECDSAPub(s.PublicKey))[4:])
	fmt.Printf("Address: %s\n", s.Address)
	return nodeURL, s
}

// getVerifiedSignedUserOp returns a signed UserOperation with a signature that has been verified by the private key.
func getVerifiedSignedUserOp(userOp *userop.UserOperation, privateKey *ecdsa.PrivateKey, publicKey *ecdsa.PublicKey, chainID *big.Int) *userop.UserOperation {
	userOp.Signature = getSignature(userOp, privateKey, chainID)

	// Verify the signature
	if verifySignature(userOp, publicKey, chainID) {
		println("Signature verified")
	} else {
		panic("Signature is invalid")
	}

	return userOp
}

func getSignature(userOp *userop.UserOperation, privateKey *ecdsa.PrivateKey, chainID *big.Int) []byte {
	userOpHash := userOp.GetUserOpHash(common.HexToAddress(entrypointAddrV060), chainID).Bytes()

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

func verifySignature(userOp *userop.UserOperation, publicKey *ecdsa.PublicKey, chainID *big.Int) bool {
	userOpHash := userOp.GetUserOpHash(common.HexToAddress(entrypointAddrV060), chainID).Bytes()

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

func getNodeIDs(nodeURL string, address common.Address) (nonce *big.Int, chainID *big.Int, err error) {
	// Initialize a client instance to interact with the Ethereum network
	client, err := ethclient.Dial(nodeURL)
	if err != nil {
		panic(fmt.Errorf("failed to connect to the Ethereum client: %w", err))
	}
	defer client.Close()

	// Retrieve the next nonce to be used
	nonceInt, err := client.PendingNonceAt(context.Background(), address)
	if err != nil {
		panic(fmt.Errorf("failed to retrieve the nonce: %w", err))
	}
	fmt.Printf("Next nonce for address %s: %d\n", address.Hex(), nonceInt)

	nonce = big.NewInt(int64(nonceInt))

	// Retrieve the chain ID
	chainID, err = client.NetworkID(context.Background())
	if err != nil {
		panic(fmt.Errorf("failed to retrieve the chain ID: %w", err))
	}
	println("Chain ID:", chainID.String())

	return nonce, chainID, nil
}

// Uncomment when testing signature verifications
// testVerifyingSignature verifies that the private key generates a signature that can be verified by the public key.
// func testVerifyingSignature(privateKey *ecdsa.PrivateKey) {
// 	publicKeyECDSA, ok := privateKey.Public().(*ecdsa.PublicKey)
// 	if !ok {
// 		panic("error casting public key to ECDSA")
// 	}
// 	address := crypto.PubkeyToAddress(*publicKeyECDSA)
//
// 	// Sample message
// 	message := "Hello, Ethereum!"
// 	prefixedHash := crypto.Keccak256Hash([]byte(fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", len(message), message)))
//
// 	// Sign the message
// 	signature, err := crypto.Sign(prefixedHash.Bytes(), privateKey)
// 	if err != nil {
// 		panic(err)
// 	}
//
// 	// Normalize S value
// 	sValue := big.NewInt(0).SetBytes(signature[32:64])
// 	// Curve order for secp256k1
// 	secp256k1N := crypto.S256().Params().N
// 	if sValue.Cmp(new(big.Int).Rsh(secp256k1N, 1)) > 0 {
// 		sValue.Sub(secp256k1N, sValue)
// 		copy(signature[32:64], sValue.Bytes())
// 	}
//
// 	// Recover the public key without adjusting V
// 	recoveredPubKey, err := crypto.SigToPub(prefixedHash.Bytes(), signature)
// 	if err != nil {
// 		panic(err)
// 	}
// 	recoveredAddress := crypto.PubkeyToAddress(*recoveredPubKey)
//
// 	fmt.Printf("Original Address: %s\n", address.Hex())
// 	fmt.Printf("Recovered Address: %s\n", recoveredAddress.Hex())
//
// 	// Check if the recovered address matches the original address
// 	if address.Hex() == recoveredAddress.Hex() {
// 		println("Signature valid, recovered address matches the original address")
// 	} else {
// 		panic("Invalid signature, recovered address does not match")
// 	}
// }
