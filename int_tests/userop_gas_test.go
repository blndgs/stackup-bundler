package int_tests

import (
	"bufio"
	"context"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/stretchr/testify/assert"
)

const LocalGeth = "http://localhost:8545"

var client *ethclient.Client

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

	client = connectToGeth()
	defer client.Close()

	// Run tests
	code := m.Run()

	// Terminate Geth
	if err := gethCmd.Process.Kill(); err != nil {
		println("Failed to kill Geth process:", err.Error())
		os.Exit(1)
	}

	os.Exit(code)
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
	client, err = ethclient.Dial(LocalGeth)
	if err != nil {
		panic("Failed to connect to Geth: " + err.Error())
	}

	return client
}

func TestBalance(t *testing.T) {
	addr := common.HexToAddress("0x0A7199a96fdf0252E09F76545c1eF2be3692F46b")
	balance, err := client.BalanceAt(context.Background(), addr, nil)
	assert.NoError(t, err, "Failed to retrieve balance")
	assert.NotNil(t, balance, "Balance should not be nil")
	assert.Equal(t, balance.String(), "1000000000000000000", "Balance should be greater than 0")
}
