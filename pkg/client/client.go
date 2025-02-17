// Package client provides the mediator for processing incoming UserOperations to the bundler.
package client

import (
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/go-logr/logr"

	"github.com/stackup-wallet/stackup-bundler/internal/logger"
	"github.com/stackup-wallet/stackup-bundler/pkg/entrypoint/filter"
	"github.com/stackup-wallet/stackup-bundler/pkg/gas"
	"github.com/stackup-wallet/stackup-bundler/pkg/mempool"
	"github.com/stackup-wallet/stackup-bundler/pkg/modules"
	"github.com/stackup-wallet/stackup-bundler/pkg/modules/noop"
	"github.com/stackup-wallet/stackup-bundler/pkg/state"
	"github.com/stackup-wallet/stackup-bundler/pkg/userop"
)

// Client controls the end to end process of adding incoming UserOperations to the mempool. It also
// implements the required RPC methods as specified in EIP-4337.
type Client struct {
	mempool              *mempool.Mempool
	ov                   *gas.Overhead
	chainID              *big.Int
	supportedEntryPoints []common.Address
	userOpHandler        modules.UserOpHandlerFunc
	logger               logr.Logger
	getUserOpReceipt     GetUserOpReceiptFunc
	getGasPrices         GetGasPricesFunc
	getGasEstimate       GetGasEstimateFunc
	getUserOpByHash      GetUserOpByHashFunc
}

// New initializes a new ERC-4337 client which can be extended with modules for validating UserOperations
// that are allowed to be added to the mempool.
func New(
	mempool *mempool.Mempool,
	ov *gas.Overhead,
	chainID *big.Int,
	supportedEntryPoints []common.Address,
) *Client {
	return &Client{
		mempool:              mempool,
		ov:                   ov,
		chainID:              chainID,
		supportedEntryPoints: supportedEntryPoints,
		userOpHandler:        noop.UserOpHandler,
		logger:               logger.NewZeroLogr().WithName("client"),
		getUserOpReceipt:     getUserOpReceiptNoop(),
		getGasPrices:         getGasPricesNoop(),
		getGasEstimate:       getGasEstimateNoop(),
		getUserOpByHash:      getUserOpByHashNoop(),
	}
}

func (i *Client) parseEntryPointAddress(ep string) (common.Address, error) {
	for _, addr := range i.supportedEntryPoints {
		if common.HexToAddress(ep) == addr {
			return addr, nil
		}
	}

	return common.Address{}, errors.New("entryPoint: Implementation not supported")
}

// UseLogger defines the logger object used by the Client instance based on the go-logr/logr interface.
func (i *Client) UseLogger(logger logr.Logger) {
	i.logger = logger.WithName("client")
}

// UseModules defines the UserOpHandlers to process a userOp after it has gone through the standard checks.
func (i *Client) UseModules(handlers ...modules.UserOpHandlerFunc) {
	i.userOpHandler = modules.ComposeUserOpHandlerFunc(handlers...)
}

// SetGetUserOpReceiptFunc defines a general function for fetching a UserOpReceipt given a userOpHash and
// EntryPoint address. This function is called in *Client.GetUserOperationReceipt.
func (i *Client) SetGetUserOpReceiptFunc(fn GetUserOpReceiptFunc) {
	i.getUserOpReceipt = fn
}

// SetGetGasPricesFunc defines a general function for fetching values for maxFeePerGas and
// maxPriorityFeePerGas. This function is called in *Client.EstimateUserOperationGas if given fee values are
// 0.
func (i *Client) SetGetGasPricesFunc(fn GetGasPricesFunc) {
	i.getGasPrices = fn
}

// SetGetGasEstimateFunc defines a general function for fetching an estimate for verificationGasLimit and
// callGasLimit given a userOp and EntryPoint address. This function is called in
// *Client.EstimateUserOperationGas.
func (i *Client) SetGetGasEstimateFunc(fn GetGasEstimateFunc) {
	i.getGasEstimate = fn
}

// SetGetUserOpByHashFunc defines a general function for fetching a userOp given a userOpHash, EntryPoint
// address, and chain ID. This function is called in *Client.GetUserOperationByHash.
func (i *Client) SetGetUserOpByHashFunc(fn GetUserOpByHashFunc) {
	i.getUserOpByHash = fn
}

// SendUserOperation implements the method call for eth_sendUserOperation.
// It returns true if userOp was accepted otherwise returns an error.
func (i *Client) SendUserOperation(op map[string]any, ep string) (string, error) {
	// Init logger
	l := i.logger.WithName("eth_sendUserOperation")

	// Check EntryPoint and userOp is valid.
	epAddr, err := i.parseEntryPointAddress(ep)
	if err != nil {
		l.Error(err, "eth_sendUserOperation error")
		return "", err
	}
	l = l.
		WithValues("entrypoint", epAddr.String()).
		WithValues("chain_id", i.chainID.String())

	userOp, err := userop.New(op)
	if err != nil {
		l.Error(err, "eth_sendUserOperation error")
		return "", err
	}
	hash := userOp.GetUserOpHash(epAddr, i.chainID)
	l = l.WithValues("userop_hash", hash)

	// Fetch any pending UserOperations in the mempool by the same sender
	penOps, err := i.mempool.GetOps(epAddr, userOp.Sender)
	if err != nil {
		l.Error(err, "eth_sendUserOperation error")
		return "", err
	}

	// Run through client module stack.
	ctx := modules.NewUserOpHandlerContext(userOp, penOps, epAddr, i.chainID)
	if err := i.userOpHandler(ctx); err != nil {
		l.Error(err, "eth_sendUserOperation error")
		return "", err
	}

	// Add userOp to mempool.
	if err := i.mempool.AddOp(epAddr, ctx.UserOp); err != nil {
		l.Error(err, "eth_sendUserOperation error")
		return "", err
	}

	l.Info("eth_sendUserOperation ok")
	return hash.String(), nil
}

// EstimateUserOperationGas returns estimates for PreVerificationGas, VerificationGasLimit, and CallGasLimit
// given a UserOperation, EntryPoint address, and state OverrideSet. The signature field and current gas
// values will not be validated although there should be dummy values in place for the most reliable results
// (e.g. a signature with the correct length).
func (i *Client) EstimateUserOperationGas(
	op map[string]any,
	ep string,
	os map[string]any,
) (*gas.GasEstimates, error) {
	// Init logger
	l := i.logger.WithName("eth_estimateUserOperationGas")

	// Check EntryPoint and userOp is valid.
	epAddr, err := i.parseEntryPointAddress(ep)
	if err != nil {
		l.Error(err, "eth_estimateUserOperationGas error")
		return nil, err
	}
	l = l.
		WithValues("entrypoint", epAddr.String()).
		WithValues("chain_id", i.chainID.String())

	userOp, err := userop.New(op)
	if err != nil {
		l.Error(err, "eth_estimateUserOperationGas error")
		return nil, err
	}
	hash := userOp.GetUserOpHash(epAddr, i.chainID)
	l = l.WithValues("userop_hash", hash)

	// Parse state override set. If paymaster is not included and sender overrides are not set, default to
	// overriding sender balance to max uint96. This ensures gas estimation is not blocked by insufficient
	// funds.
	sos, err := state.ParseOverrideData(os)
	if err != nil {
		l.Error(err, "eth_estimateUserOperationGas error")
		return nil, err
	}
	if userOp.GetPaymaster() == common.HexToAddress("0x") {
		sos = state.WithMaxBalanceOverride(userOp.Sender, sos)
	}

	// Override op with suggested gas prices if maxFeePerGas is 0. This allows for more reliable gas
	// estimations upstream. The default balance override also ensures simulations won't revert on
	// insufficient funds.
	if userOp.MaxFeePerGas.Cmp(common.Big0) != 1 {
		gp, err := i.getGasPrices()
		if err != nil {
			l.Error(err, "eth_estimateUserOperationGas error")
			return nil, err
		}
		userOp.MaxFeePerGas = gp.MaxFeePerGas
		userOp.MaxPriorityFeePerGas = gp.MaxPriorityFeePerGas
	}

	// Estimate gas limits
	vg, cg, err := i.getGasEstimate(epAddr, userOp, sos)
	if err != nil {
		l.Error(err, "eth_estimateUserOperationGas error")
		return nil, err
	}

	// Calculate PreVerificationGas
	pvg, err := i.ov.CalcPreVerificationGasWithBuffer(userOp)
	if err != nil {
		l.Error(err, "eth_estimateUserOperationGas error")
		return nil, err
	}

	l.Info("eth_estimateUserOperationGas ok")
	return &gas.GasEstimates{
		PreVerificationGas:   pvg,
		VerificationGasLimit: big.NewInt(int64(vg)),
		CallGasLimit:         big.NewInt(int64(cg)),

		// TODO: Deprecate in v0.7
		VerificationGas: big.NewInt(int64(vg)),
	}, nil
}

// GetUserOperationReceipt fetches a UserOperation receipt based on a userOpHash returned by
// *Client.SendUserOperation.
func (i *Client) GetUserOperationReceipt(
	hash string,
) (*filter.UserOperationReceipt, error) {
	// Init logger
	l := i.logger.WithName("eth_getUserOperationReceipt").WithValues("userop_hash", hash)

	pooled, err := i.mempool.HasUserOpHash(hash)
	if err != nil {
		l.Error(err, "mempool.HashUserOpHash error")
		return nil, err
	}

	if pooled {
		// UserOperation is in mempool
		l.Info("mempool.HasUserOpHash returned true: " + hash)
		var r filter.UserOperationReceipt
		r.Nonce = "-1"
		return &r, nil
	}

	ev, err := i.getUserOpReceipt(hash, i.supportedEntryPoints[0])
	if err != nil {
		l.Error(err, "eth_getUserOperationReceipt error")
		return nil, err
	}

	l.Info("eth_getUserOperationReceipt ok")
	return ev, nil
}

// GetUserOperationByHash returns a UserOperation based on a given userOpHash returned by
// *Client.SendUserOperation.
func (i *Client) GetUserOperationByHash(hash string) (*filter.HashLookupResult, error) {
	// Init logger
	l := i.logger.WithName("eth_getUserOperationByHash").WithValues("userop_hash", hash)

	res, err := i.getUserOpByHash(hash, i.supportedEntryPoints[0], i.chainID)
	if err != nil {
		l.Error(err, "eth_getUserOperationByHash error")
		return nil, err
	}

	return res, nil
}

// SupportedEntryPoints implements the method call for eth_supportedEntryPoints. It returns the array of
// EntryPoint addresses that is supported by the client. The first address in the array is the preferred
// EntryPoint.
func (i *Client) SupportedEntryPoints() ([]string, error) {
	slc := []string{}
	for _, ep := range i.supportedEntryPoints {
		slc = append(slc, ep.String())
	}

	return slc, nil
}

// ChainID implements the method call for eth_chainId. It returns the current chainID used by the client.
// This method is used to validate that the client's chainID is in sync with the caller.
func (i *Client) ChainID() (string, error) {
	return hexutil.EncodeBig(i.chainID), nil
}
