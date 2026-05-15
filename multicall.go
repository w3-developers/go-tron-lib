package tron

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
)

const multicallABIJSON = `[{"inputs":[{"components":[{"internalType":"address","name":"target","type":"address"},{"internalType":"bytes","name":"callData","type":"bytes"}],"internalType":"struct Multicall.Call[]","name":"calls","type":"tuple[]"}],"name":"aggregate","outputs":[{"internalType":"uint256","name":"blockNumber","type":"uint256"},{"internalType":"bytes[]","name":"returnData","type":"bytes[]"}],"stateMutability":"nonpayable","type":"function"}]`
const erc20BalanceOfABIJSON = `[{"inputs":[{"internalType":"address","name":"account","type":"address"}],"name":"balanceOf","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"}]`
const getEthBalanceABIJSON = `[{"inputs":[{"internalType":"address","name":"addr","type":"address"}],"name":"getEthBalance","outputs":[{"internalType":"uint256","name":"balance","type":"uint256"}],"stateMutability":"view","type":"function"}]`

type Multicall struct {
	multicallAddress string
	c                *Client
}

func (c *Client) NewMulticall(multicallAddress string) *Multicall {
	return &Multicall{c: c, multicallAddress: multicallAddress}
}

type MulticallCall struct {
	Target   string
	CallData []byte
}

func (c *Multicall) AggregateMulticall(
	ctx context.Context,
	calls []MulticallCall,
) ([][]byte, error) {
	if len(calls) == 0 {
		return nil, nil
	}

	mcABI, err := abi.JSON(strings.NewReader(multicallABIJSON))
	if err != nil {
		return nil, fmt.Errorf("parse multicall abi: %w", err)
	}

	type Call struct {
		Target   [20]byte
		CallData []byte
	}

	packedCalls := make([]Call, 0, len(calls))
	for i, cl := range calls {
		targetHex, err := TronBase58ToHex(cl.Target)
		if err != nil {
			return nil, fmt.Errorf("invalid target address[%d]: %w", i, err)
		}
		targetRaw20, err := hex.DecodeString(targetHex[2:])
		if err != nil {
			return nil, fmt.Errorf("decode target hex[%d]: %w", i, err)
		}

		var target20 [20]byte
		copy(target20[:], targetRaw20)

		packedCalls = append(packedCalls, Call{
			Target:   target20,
			CallData: cl.CallData,
		})
	}

	input, err := mcABI.Pack("aggregate", packedCalls)
	if err != nil {
		return nil, fmt.Errorf("pack aggregate: %w", err)
	}

	parameter := hex.EncodeToString(input[4:])

	raw, err := c.c.TriggerConstantContract(ctx, TriggerConstantContractReq{
		OwnerAddress:    c.multicallAddress,
		ContractAddress: c.multicallAddress,
		Function:        "aggregate((address,bytes)[])",
		Parameter:       parameter,
		Visible:         c.c.visible,
	})
	if err != nil {
		return nil, fmt.Errorf("trigger constant contract: %w", err)
	}

	var result struct {
		ConstantResult []string `json:"constant_result"`
		Result         struct {
			Result bool `json:"result"`
		} `json:"result"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("decode rpc response: %w", err)
	}

	if len(result.ConstantResult) == 0 {
		return nil, fmt.Errorf("empty constant_result: %s", raw)
	}

	rawBytes, err := hex.DecodeString(result.ConstantResult[0])
	if err != nil {
		return nil, fmt.Errorf("decode constant_result hex: %w", err)
	}

	output, err := mcABI.Unpack("aggregate", rawBytes)
	if err != nil {
		return nil, fmt.Errorf("unpack aggregate: %w", err)
	}

	returnData := output[1].([][]byte)

	if len(returnData) != len(calls) {
		return nil, fmt.Errorf("multicall returned %d results for %d calls", len(returnData), len(calls))
	}

	return returnData, nil
}

func UnpackMulticallResults(results [][]byte, abiJSON string, functionName string) ([]interface{}, error) {
	contractABI, err := abi.JSON(strings.NewReader(abiJSON))
	if err != nil {
		return nil, fmt.Errorf("parse abi: %w", err)
	}

	unpacked := make([]interface{}, len(results))
	for i, result := range results {
		values, err := contractABI.Unpack(functionName, result)
		if err != nil {
			return nil, fmt.Errorf("unpack %s[%d]: %w", functionName, i, err)
		}
		unpacked[i] = values
	}

	return unpacked, nil
}

func PackCallData(abiJSON string, functionName string, args ...interface{}) ([]byte, error) {
	contractABI, err := abi.JSON(strings.NewReader(abiJSON))
	if err != nil {
		return nil, fmt.Errorf("parse abi: %w", err)
	}

	callData, err := contractABI.Pack(functionName, args...)
	if err != nil {
		return nil, fmt.Errorf("pack %s: %w", functionName, err)
	}

	return callData, nil
}

func (c *Multicall) BalanceOf(
	ctx context.Context,
	tokenAddress string,
	addresses []string,
) ([]*big.Int, error) {
	balances := make([]*big.Int, len(addresses))
	if len(addresses) == 0 {
		return balances, nil
	}

	calls := make([]MulticallCall, len(addresses))
	tokenHex, err := TronBase58ToHex(tokenAddress)
	if err != nil {
		return nil, fmt.Errorf("invalid token address: %w", err)
	}
	tokenRaw20, _ := hex.DecodeString(tokenHex[2:])

	var token20 [20]byte
	copy(token20[:], tokenRaw20)

	for i, address := range addresses {
		wHex, err := TronBase58ToHex(address)
		if err != nil {
			return nil, fmt.Errorf("invalid address %s: %w", address, err)
		}
		wRaw20, err := hex.DecodeString(wHex[2:])
		if err != nil {
			return nil, fmt.Errorf("decode address hex %s: %w", address, err)
		}
		if len(wRaw20) != 20 {
			return nil, fmt.Errorf("invalid address length %s: got %d bytes, want 20", address, len(wRaw20))
		}

		callData, err := PackCallData(erc20BalanceOfABIJSON, "balanceOf", common.BytesToAddress(wRaw20))
		if err != nil {
			return nil, fmt.Errorf("pack balanceOf: %w", err)
		}

		calls[i] = MulticallCall{
			Target:   tokenAddress,
			CallData: callData,
		}
	}

	returnData, err := c.AggregateMulticall(ctx, calls)
	if err != nil {
		return nil, err
	}

	unpacked, err := UnpackMulticallResults(returnData, erc20BalanceOfABIJSON, "balanceOf")
	if err != nil {
		return nil, err
	}

	for i, values := range unpacked {
		valuesSlice := values.([]interface{})
		if len(valuesSlice) == 0 {
			return nil, fmt.Errorf("empty unpacked result[%d]", i)
		}
		balance, ok := valuesSlice[0].(*big.Int)
		if !ok {
			return nil, fmt.Errorf("invalid balance type[%d]", i)
		}
		balances[i] = balance
	}

	return balances, nil
}

func (c *Multicall) BalanceAt(
	ctx context.Context,
	addresses []string,
) ([]*big.Int, error) {
	balances := make([]*big.Int, len(addresses))
	if len(addresses) == 0 {
		return balances, nil
	}

	calls := make([]MulticallCall, len(addresses))

	for i, address := range addresses {
		wHex, err := TronBase58ToHex(address)
		if err != nil {
			return nil, fmt.Errorf("invalid address %s: %w", address, err)
		}
		wRaw20, err := hex.DecodeString(wHex[2:])
		if err != nil {
			return nil, fmt.Errorf("decode address hex %s: %w", address, err)
		}
		if len(wRaw20) != 20 {
			return nil, fmt.Errorf("invalid address length %s: got %d bytes, want 20", address, len(wRaw20))
		}

		callData, err := PackCallData(getEthBalanceABIJSON, "getEthBalance", common.BytesToAddress(wRaw20))
		if err != nil {
			return nil, fmt.Errorf("pack balanceOf: %w", err)
		}

		calls[i] = MulticallCall{
			Target:   c.multicallAddress,
			CallData: callData,
		}

		fmt.Printf("call[%d]: %s\n", i, calls[i].Target)
		fmt.Printf("call[%d]: %s\n", i, calls[i].CallData)
	}

	returnData, err := c.AggregateMulticall(ctx, calls)
	if err != nil {
		return nil, err
	}

	unpacked, err := UnpackMulticallResults(returnData, getEthBalanceABIJSON, "getEthBalance")
	if err != nil {
		return nil, err
	}

	for i, values := range unpacked {
		valuesSlice := values.([]interface{})
		if len(valuesSlice) == 0 {
			return nil, fmt.Errorf("empty unpacked result[%d]", i)
		}
		balance, ok := valuesSlice[0].(*big.Int)
		if !ok {
			return nil, fmt.Errorf("invalid balance type[%d]", i)
		}
		balances[i] = balance
	}

	return balances, nil
}
