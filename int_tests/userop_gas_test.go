package int_tests

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/joho/godotenv"
	"github.com/stretchr/testify/assert"
)

const LocalGeth = "http://localhost:8545"

var (
	ethclnt   *ethclient.Client
	signerKey *ecdsa.PrivateKey
)

func TestMain(m *testing.M) {
	// Check if Geth is in the user's path
	_, err := exec.LookPath("geth")
	if err != nil {
		log.Fatal("Geth is not installed or not in the system path. Please install Geth and try again.")
	}

	// Start Geth
	gethCmd := exec.Command("geth", "--verbosity", "5",
		"--http.vhosts", "'*,localhost,host.docker.internal'",
		"--http", "--http.api", "eth,net,web3,debug",
		"--http.corsdomain", "'*'",
		"--http.addr", "0.0.0.0",
		"--nodiscover", "--maxpeers", "0", "--mine",
		"--networkid", "1337",
		"--dev",
		"--allow-insecure-unlock",
		"--rpc.allow-unprotected-txs",
		"--miner.gaslimit", "12000000")

	gethStdout, _ := gethCmd.StdoutPipe()
	gethStderr, _ := gethCmd.StderrPipe()
	if err := gethCmd.Start(); err != nil {
		log.Fatal("Failed to start Geth:", err)
	}
	defer terminateGeth(gethCmd)

	setSignerKey()

	// Asynchronously log Geth output
	go logOutput(gethStdout)
	go logOutput(gethStderr)

	// Wait for Geth to be ready
	waitForGeth()

	// Execute the sendtx.sh script
	err = executeSendTxScript()
	if err != nil {
		log.Fatalf("Failed to execute sendtx.sh script: %v", err)
	}

	ethclnt = connectToGeth()
	defer ethclnt.Close()

	// Run tests
	code := m.Run()

	os.Exit(code)
}

func terminateGeth(gethCmd *exec.Cmd) {
	if err := gethCmd.Process.Kill(); err != nil {
		log.Println("Failed to kill Geth process:", err.Error())
	}
}

func setSignerKey() {
	if err := godotenv.Load(); err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}

	// Retrieve the prv value
	var privateKeyStr string
	if privateKeyStr = os.Getenv("PRIVATE_KEY"); privateKeyStr == "" {
		log.Fatal("PRIVATE_KEY environment variable is not set")
	}
	// Convert PRIVATE_KEY to ECDSA private key
	var err error
	signerKey, err = crypto.HexToECDSA(privateKeyStr)
	if err != nil {
		log.Fatalf("Invalid PRIVATE_KEY: %v", err)
	}
}

func executeSendTxScript() error {
	cmd := exec.Command("./sendtx.sh")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func logOutput(pipe io.ReadCloser) {
	scanner := bufio.NewScanner(pipe)
	for scanner.Scan() {
		log.Println(scanner.Text())
	}
}

func waitForGeth() {
	ticker := time.NewTicker(time.Millisecond * 300)
	defer ticker.Stop()

	timeout := time.After(30 * time.Second)

	for {
		select {
		case <-ticker.C:
			resp, err := http.Get(LocalGeth)
			if err == nil {
				resp.Body.Close()
				return
			}
		case <-timeout:
			log.Fatal("Geth did not start in the expected time")
		}
	}
}

func connectToGeth() *ethclient.Client {
	var err error
	ethclnt, err = ethclient.Dial(LocalGeth)
	if err != nil {
		panic("Failed to connect to Geth: " + err.Error())
	}

	return ethclnt
}

func TestBalance(t *testing.T) {
	addr := common.HexToAddress("0x0A7199a96fdf0252E09F76545c1eF2be3692F46b")
	balance, err := ethclnt.BalanceAt(context.Background(), addr, nil)
	assert.NoError(t, err, "Failed to retrieve balance")
	assert.NotNil(t, balance, "Balance should not be nil")
	assert.Equal(t, balance.String(), "1000000000000000000", "Balance should be greater than 0")
}
