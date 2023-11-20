// Package jsonrpc implements Gin middleware for handling JSON-RPC requests via HTTP.
package jsonrpc

import (
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"reflect"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/gin-gonic/gin"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/stackup-wallet/stackup-bundler/pkg/errors"
)

func jsonrpcError(c *gin.Context, code int, message string, data any, id *float64) {
	c.JSON(http.StatusOK, gin.H{
		"jsonrpc": "2.0",
		"error": gin.H{
			"code":    code,
			"message": message,
			"data":    data,
		},
		"id": id,
	})
	c.Abort()
}

// Controller returns a custom Gin middleware that handles incoming JSON-RPC requests via HTTP. It maps the
// RPC method name to struct methods on the given api. For example, if the RPC request has the method field
// set to "namespace_methodName" then the controller will make a call to api.Namespace_methodName with the
// params spread as arguments.
//
// If request is valid it will also set the data on the Gin context with the key "json-rpc-request".
func Controller(api interface{}, rpcClient *rpc.Client, ethRPCClient *ethclient.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method != "POST" {
			jsonrpcError(c, -32700, "Parse error", "POST method excepted", nil)
			return
		}

		if c.Request.Body == nil {
			jsonrpcError(c, -32700, "Parse error", "No POST data", nil)
			return
		}

		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			jsonrpcError(c, -32700, "Parse error", "Error while reading request body", nil)
			return
		}

		data := make(map[string]any)
		err = json.Unmarshal(body, &data)
		if err != nil {
			jsonrpcError(c, -32700, "Parse error", "Error parsing json request", nil)
			return
		}

		id, ok := data["id"].(float64)
		if !ok {
			jsonrpcError(c, -32600, "Invalid Request", "No or invalid 'id' in request", nil)
			return
		}

		if data["jsonrpc"] != "2.0" {
			jsonrpcError(c, -32600, "Invalid Request", "Version of jsonrpc is not 2.0", &id)
			return
		}

		method, ok := data["method"].(string)
		if !ok {
			jsonrpcError(c, -32600, "Invalid Request", "No or invalid 'method' in request", &id)
			return
		}

		if isStdEthereumRPCMethod(method) {
			fmt.Println("Method:", method)
			// Proxy the request to the Ethereum node
			routeStdEthereumRPCRequest(c, method, rpcClient, ethRPCClient, data)
			return
		}

		params, ok := data["params"].([]interface{})
		if !ok {
			jsonrpcError(c, -32602, "Invalid params", "No or invalid 'params' in request", &id)
			return
		}

		call := reflect.ValueOf(api).MethodByName(cases.Title(language.Und, cases.NoLower).String(method))
		if !call.IsValid() {
			jsonrpcError(c, -32601, "Method not found", "Method not found", &id)
			return
		}

		if call.Type().NumIn() != len(params) {
			jsonrpcError(c, -32602, "Invalid params", "Invalid number of params", &id)
			return
		}

		args := make([]reflect.Value, len(params))
		for i, arg := range params {
			switch call.Type().In(i).Kind() {
			case reflect.Float32:
				val, ok := arg.(float32)
				if !ok {
					jsonrpcError(
						c,
						-32602,
						"Invalid params",
						fmt.Sprintf("Param [%d] can't be converted to %v", i, call.Type().In(i).String()),
						&id,
					)
					return
				}
				args[i] = reflect.ValueOf(val)

			case reflect.Float64:
				val, ok := arg.(float64)
				if !ok {
					jsonrpcError(
						c,
						-32602,
						"Invalid params",
						fmt.Sprintf("Param [%d] can't be converted to %v", i, call.Type().In(i).String()),
						&id,
					)
					return
				}
				args[i] = reflect.ValueOf(val)

			case reflect.Int:
				val, ok := arg.(int)
				if !ok {
					var fval float64
					fval, ok = arg.(float64)
					if ok {
						val = int(fval)
					}
				}

				if !ok {
					jsonrpcError(
						c,
						-32602,
						"Invalid params",
						fmt.Sprintf("Param [%d] can't be converted to %v", i, call.Type().In(i).String()),
						&id,
					)
					return
				}
				args[i] = reflect.ValueOf(val)

			case reflect.Int8:
				val, ok := arg.(int8)
				if !ok {
					var fval float64
					fval, ok = arg.(float64)
					if ok {
						val = int8(fval)
					}
				}
				if !ok {
					jsonrpcError(
						c,
						-32602,
						"Invalid params",
						fmt.Sprintf("Param [%d] can't be converted to %v", i, call.Type().In(i).String()),
						&id,
					)
					return
				}
				args[i] = reflect.ValueOf(val)

			case reflect.Int16:
				val, ok := arg.(int16)
				if !ok {
					var fval float64
					fval, ok = arg.(float64)
					if ok {
						val = int16(fval)
					}
				}
				if !ok {
					jsonrpcError(
						c,
						-32602,
						"Invalid params",
						fmt.Sprintf("Param [%d] can't be converted to %v", i, call.Type().In(i).String()),
						&id,
					)
					return
				}
				args[i] = reflect.ValueOf(val)

			case reflect.Int32:
				val, ok := arg.(int32)
				if !ok {
					var fval float64
					fval, ok = arg.(float64)
					if ok {
						val = int32(fval)
					}
				}
				if !ok {
					jsonrpcError(
						c,
						-32602,
						"Invalid params",
						fmt.Sprintf("Param [%d] can't be converted to %v", i, call.Type().In(i).String()),
						&id,
					)
					return
				}
				args[i] = reflect.ValueOf(val)

			case reflect.Int64:
				val, ok := arg.(int64)
				if !ok {
					var fval float64
					fval, ok = arg.(float64)
					if ok {
						val = int64(fval)
					}
				}
				if !ok {
					jsonrpcError(
						c,
						-32602,
						"Invalid params",
						fmt.Sprintf("Param [%d] can't be converted to %v", i, call.Type().In(i).String()),
						&id,
					)
					return
				}
				args[i] = reflect.ValueOf(val)

			case reflect.Interface:
				args[i] = reflect.ValueOf(arg)

			case reflect.Map:
				val, ok := arg.(map[string]any)
				if !ok {
					jsonrpcError(
						c,
						-32602,
						"Invalid params",
						fmt.Sprintf("Param [%d] can't be converted to %v", i, call.Type().In(i).String()),
						&id,
					)
					return
				}
				args[i] = reflect.ValueOf(val)

			case reflect.Slice:
				val, ok := arg.([]interface{})
				if !ok {
					jsonrpcError(
						c,
						-32602,
						"Invalid params",
						fmt.Sprintf("Param [%d] can't be converted to %v", i, call.Type().In(i).String()),
						&id,
					)
					return
				}
				args[i] = reflect.ValueOf(val)

			case reflect.String:
				val, ok := arg.(string)
				if !ok {
					jsonrpcError(
						c,
						-32602,
						"Invalid params",
						fmt.Sprintf("Param [%d] can't be converted to %v", i, call.Type().In(i).String()),
						&id,
					)
					return
				}
				args[i] = reflect.ValueOf(val)

			case reflect.Uint:
				val, ok := arg.(uint)
				if !ok {
					var fval float64
					fval, ok = arg.(float64)
					if ok {
						val = uint(fval)
					}
				}
				if !ok {
					jsonrpcError(
						c,
						-32602,
						"Invalid params",
						fmt.Sprintf("Param [%d] can't be converted to %v", i, call.Type().In(i).String()),
						&id,
					)
					return
				}
				args[i] = reflect.ValueOf(val)

			case reflect.Uint8:
				val, ok := arg.(uint8)
				if !ok {
					var fval float64
					fval, ok = arg.(float64)
					if ok {
						val = uint8(fval)
					}
				}
				if !ok {
					jsonrpcError(
						c,
						-32602,
						"Invalid params",
						fmt.Sprintf("Param [%d] can't be converted to %v", i, call.Type().In(i).String()),
						&id,
					)
					return
				}
				args[i] = reflect.ValueOf(val)

			case reflect.Uint16:
				val, ok := arg.(uint16)
				if !ok {
					var fval float64
					fval, ok = arg.(float64)
					if ok {
						val = uint16(fval)
					}
				}
				if !ok {
					jsonrpcError(
						c,
						-32602,
						"Invalid params",
						fmt.Sprintf("Param [%d] can't be converted to %v", i, call.Type().In(i).String()),
						&id,
					)
					return
				}
				args[i] = reflect.ValueOf(val)

			case reflect.Uint32:
				val, ok := arg.(uint32)
				if !ok {
					var fval float64
					fval, ok = arg.(float64)
					if ok {
						val = uint32(fval)
					}
				}
				if !ok {
					jsonrpcError(
						c,
						-32602,
						"Invalid params",
						fmt.Sprintf("Param [%d] can't be converted to %v", i, call.Type().In(i).String()),
						&id,
					)
					return
				}
				args[i] = reflect.ValueOf(val)

			case reflect.Uint64:
				val, ok := arg.(uint64)
				if !ok {
					var fval float64
					fval, ok = arg.(float64)
					if ok {
						val = uint64(fval)
					}
				}
				if !ok {
					jsonrpcError(
						c,
						-32602,
						"Invalid params",
						fmt.Sprintf("Param [%d] can't be converted to %v", i, call.Type().In(i).String()),
						&id,
					)
					return
				}
				args[i] = reflect.ValueOf(val)

			default:
				if !ok {
					jsonrpcError(c, -32603, "Internal error", "Invalid method definition", &id)
					return
				}
			}
		}

		c.Set("json-rpc-request", data)
		result := call.Call(args)
		if err, ok := result[len(result)-1].Interface().(error); ok && err != nil {
			rpcErr, ok := err.(*errors.RPCError)

			if ok {
				jsonrpcError(c, rpcErr.Code(), rpcErr.Error(), rpcErr.Data(), &id)
			} else {
				jsonrpcError(c, -32601, err.Error(), err.Error(), &id)
			}
		} else if len(result) > 0 {
			c.JSON(http.StatusOK, gin.H{
				"result":  result[0].Interface(),
				"jsonrpc": "2.0",
				"id":      id,
			})
		} else {
			c.JSON(http.StatusOK, gin.H{
				"result":  nil,
				"jsonrpc": "2.0",
				"id":      id,
			})
		}
	}
}

func isStdEthereumRPCMethod(method string) bool {
	bundlerMethods := map[string]bool{
		"eth_senduseroperation":         true,
		"eth_estimateuseroperationgas":  true,
		"eth_getuseroperationreceipt":   true,
		"eth_getuseroperationbyhash":    true,
		"eth_supportedentrypoints":      true,
		"eth_chainid":                   true,
		"debug_bundler_clearstate":      true,
		"debug_bundler_dumpmempool":     true,
		"debug_bundler_sendbundlenow":   true,
		"debug_bundler_setbundlingmode": true,
		// Add any other bundler-specific methods here
	}

	// Check if the method is NOT a bundler-specific method
	_, isBundlerMethod := bundlerMethods[strings.ToLower(method)]

	return !isBundlerMethod
}

func routeStdEthereumRPCRequest(c *gin.Context, method string, rpcClient *rpc.Client, ethClient *ethclient.Client, requestData map[string]any) {
	const ethCall = "eth_call"
	if strings.ToLower(method) == ethCall {
		handleEthCallRequest(c, ethClient, requestData)
		return
	}

	handleEthRequest(c, method, rpcClient, requestData)
}

func handleEthRequest(c *gin.Context, method string, rpcClient *rpc.Client, requestData map[string]any) {
	// Extract params and keep them in their original type
	params, ok := requestData["params"].([]interface{})
	if !ok {
		jsonrpcError(c, -32602, "Invalid params format", "Expected a slice of parameters", nil)
		return
	}

	// Prepare a slice to hold the result references based on the method requirements
	var result interface{}
	switch method {
	case "eth_getBlockByNumber":
	case "eth_maxPriorityFeePerGas":
		result = new(hexutil.Big)
	default:
		jsonrpcError(c, -32601, "Method not found", method, nil)
		return
	}

	// Call the method with the parameters
	err := rpcClient.Call(result, method, params...)
	if err != nil {
		jsonrpcError(c, -32603, "Internal error", err.Error(), nil)
		return
	}

	// Convert result to a string representation or handle based on type
	var resultStr string
	switch res := result.(type) {
	case *hexutil.Big:
		resultStr = res.String()
	default:
		jsonrpcError(c, -32603, "Unexpected result type", fmt.Sprintf("%T", result), nil)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"result":  resultStr,
		"jsonrpc": "2.0",
		"id":      requestData["id"],
	})
}

func handleEthCallRequest(c *gin.Context, ethClient *ethclient.Client, requestData map[string]any) {
	params := requestData["params"].([]interface{})

	var (
		callParams map[string]interface{}
		to         string
		data       string
		callMsg    ethereum.CallMsg
	)
	if len(params) > 0 {
		// Assuming the first param is the address and the second is the data
		// This needs to be adjusted according to the specific RPC method and parameters
		ok := false
		callParams, ok = params[0].(map[string]interface{})
		if !ok {
			jsonrpcError(c, -32602, "Invalid params", "First parameter should be a map", nil)
			return
		}

		to, ok = callParams["to"].(string)
		if !ok {
			jsonrpcError(c, -32602, "Invalid params", "Contract address (to) not provided or invalid", nil)
			return
		}

		data, ok = callParams["data"].(string)
		if !ok {
			jsonrpcError(c, -32602, "Invalid params", "Data not provided or invalid", nil)
			return
		}

		address := common.HexToAddress(to)
		callMsg = ethereum.CallMsg{
			To:   &address,
			Data: common.FromHex(data),
		}
	}

	var blockNumber *big.Int
	if len(params) > 1 {
		blockParam := params[1].(string)
		if blockParam != "latest" {
			var intBlockNumber int64
			intBlockNumber, err := strconv.ParseInt(blockParam, 10, 64)
			if err != nil {
				jsonrpcError(c, -32602, "Invalid params", "Third parameter should be a block number or 'latest'", nil)
				return
			}
			blockNumber = big.NewInt(intBlockNumber)
		}
	}

	result, err := ethClient.CallContract(c, callMsg, blockNumber)
	// The erc-4337 spec has a special case for revert errors, where the revert data is returned as the result
	const revertErrorKey = "execution reverted"
	if err != nil && err.Error() == revertErrorKey {
		strResult := extractDataFromUnexportedError(err)
		if strResult != "" {
			c.JSON(http.StatusOK, gin.H{
				"result":  strResult,
				"jsonrpc": "2.0",
				"id":      requestData["id"],
			})

			return
		}
	}

	if err != nil {
		jsonrpcError(c, -32603, "Internal error", err.Error(), nil)
		return
	}

	resultStr := "0x" + common.Bytes2Hex(result)

	c.JSON(http.StatusOK, gin.H{
		"result":  resultStr,
		"jsonrpc": "2.0",
		"id":      requestData["id"],
	})
}

// extractDataFromUnexportedError extracts the "Data" field from *rpc.jsonError that is not exported
// using reflection.
func extractDataFromUnexportedError(err error) string {
	if err == nil {
		return ""
	}

	val := reflect.ValueOf(err)
	if val.Kind() == reflect.Ptr && !val.IsNil() {
		// Assuming jsonError is a struct
		errVal := val.Elem()

		// Check if the struct has a field named "Data".
		dataField := errVal.FieldByName("Data")
		if dataField.IsValid() && dataField.CanInterface() {
			// Assuming the data field is a string
			return dataField.Interface().(string)
		}
	}

	return ""
}
