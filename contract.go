package tron

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
)

func (c *Client) BuildFunctionTx(
	ctx context.Context,
	contractAddress string,
	functionSig string,
	feeLimit int64,
	fromAddress string,
	params ...any,
) (json.RawMessage, error) {
	if strings.TrimSpace(functionSig) == "" {
		return nil, errors.New("empty function signature")
	}

	paramHex, err := BuildTRONABIParams(functionSig, params...)
	if err != nil {
		return nil, err
	}

	raw, err := c.TriggerSmartContract(ctx, TriggerSmartContractReq{
		OwnerAddress:    fromAddress,
		ContractAddress: contractAddress,
		Function:        functionSig,
		Parameter:       paramHex,
		FeeLimit:        feeLimit,
		Visible:         true,
	})
	if err != nil {
		return nil, err
	}

	var out TriggerSmartResult
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	if !out.Result.Result {
		if out.Message != "" {
			return nil, errors.New(out.Message)
		}
		return nil, errors.New("triggersmartcontract failed")
	}
	if len(out.Transaction) == 0 {
		return nil, errors.New("empty transaction")
	}

	return out.Transaction, nil
}

func BuildTRONABIParams(functionSig string, params ...any) (string, error) {
	types, err := parseFunctionSignature(functionSig)
	if err != nil {
		return "", err
	}
	if len(types) != len(params) {
		return "", fmt.Errorf("params count mismatch: expected %d, got %d", len(types), len(params))
	}

	encoded := make([]string, 0, len(params))
	for i, typ := range types {
		part, err := encodeABIParamByType(typ, params[i])
		if err != nil {
			return "", fmt.Errorf("param %d (%s): %w", i, typ, err)
		}
		encoded = append(encoded, part)
	}

	return strings.Join(encoded, ""), nil
}

func parseFunctionSignature(sig string) ([]string, error) {
	sig = strings.TrimSpace(sig)
	open := strings.Index(sig, "(")
	close := strings.LastIndex(sig, ")")
	if open < 0 || close < 0 || close < open {
		return nil, fmt.Errorf("invalid function signature: %s", sig)
	}

	inside := strings.TrimSpace(sig[open+1 : close])
	if inside == "" {
		return nil, nil
	}

	rawTypes := strings.Split(inside, ",")
	types := make([]string, 0, len(rawTypes))
	for _, t := range rawTypes {
		tt := normalizeABIType(t)
		if tt == "" {
			return nil, fmt.Errorf("empty type in signature: %s", sig)
		}
		types = append(types, tt)
	}
	return types, nil
}

func normalizeABIType(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ToLower(s)

	switch s {
	case "uint":
		return "uint256"
	case "int":
		return "int256"
	default:
		return s
	}
}

func encodeABIParamByType(typ string, v any) (string, error) {
	switch normalizeABIType(typ) {
	case "address":
		return encodeABIAddress(v)
	case "uint256":
		return encodeABIUint256(v)
	case "int256":
		return encodeABIInt256(v)
	case "bool":
		return encodeABIBool(v)
	case "bytes32":
		return encodeABIBytes32(v)
	default:
		return "", fmt.Errorf("unsupported abi type: %s", typ)
	}
}

func encodeABIBytes32(v any) (string, error) {
	bytes32, ok := v.([32]byte)
	if !ok {
		return "", fmt.Errorf("bytes32 must be [32]byte, got %T", v)
	}
	return leftPad64(hex.EncodeToString(bytes32[:])), nil
}

func encodeABIAddress(v any) (string, error) {
	addr, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("address must be string, got %T", v)
	}

	addrHex20, err := tronAddressToABIHex(addr)
	if err != nil {
		return "", err
	}

	return leftPad64(addrHex20), nil
}

func encodeABIUint256(v any) (string, error) {
	n, err := toBigInt(v)
	if err != nil {
		return "", err
	}
	if n.Sign() < 0 {
		return "", errors.New("uint256 cannot be negative")
	}
	return leftPad64(strings.TrimLeft(n.Text(16), "0")), nil
}

func encodeABIInt256(v any) (string, error) {
	n, err := toBigInt(v)
	if err != nil {
		return "", err
	}

	limit := new(big.Int).Lsh(big.NewInt(1), 255)
	if n.Cmp(limit) >= 0 || n.Cmp(new(big.Int).Neg(limit)) < 0 {
		return "", errors.New("int256 overflow")
	}

	if n.Sign() >= 0 {
		return leftPad64(strings.TrimLeft(n.Text(16), "0")), nil
	}

	mod := new(big.Int).Lsh(big.NewInt(1), 256)
	twosComplement := new(big.Int).Add(mod, n)
	return leftPad64(strings.TrimLeft(twosComplement.Text(16), "0")), nil
}

func encodeABIBool(v any) (string, error) {
	var b bool

	switch x := v.(type) {
	case bool:
		b = x
	case string:
		parsed, err := strconv.ParseBool(x)
		if err != nil {
			return "", fmt.Errorf("invalid bool string: %q", x)
		}
		b = parsed
	default:
		return "", fmt.Errorf("bool must be bool or string, got %T", v)
	}

	if b {
		return leftPad64("1"), nil
	}
	return leftPad64("0"), nil
}

func toBigInt(v any) (*big.Int, error) {
	switch x := v.(type) {
	case *big.Int:
		if x == nil {
			return nil, errors.New("nil *big.Int")
		}
		return new(big.Int).Set(x), nil
	case big.Int:
		return new(big.Int).Set(&x), nil
	case int:
		return big.NewInt(int64(x)), nil
	case int8:
		return big.NewInt(int64(x)), nil
	case int16:
		return big.NewInt(int64(x)), nil
	case int32:
		return big.NewInt(int64(x)), nil
	case int64:
		return big.NewInt(x), nil
	case uint:
		z := new(big.Int)
		z.SetUint64(uint64(x))
		return z, nil
	case uint8:
		z := new(big.Int)
		z.SetUint64(uint64(x))
		return z, nil
	case uint16:
		z := new(big.Int)
		z.SetUint64(uint64(x))
		return z, nil
	case uint32:
		z := new(big.Int)
		z.SetUint64(uint64(x))
		return z, nil
	case uint64:
		z := new(big.Int)
		z.SetUint64(x)
		return z, nil
	case string:
		x = strings.TrimSpace(x)
		if x == "" {
			return nil, errors.New("empty numeric string")
		}
		z := new(big.Int)
		if strings.HasPrefix(x, "0x") || strings.HasPrefix(x, "0X") {
			_, ok := z.SetString(x[2:], 16)
			if !ok {
				return nil, fmt.Errorf("invalid hex integer: %s", x)
			}
			return z, nil
		}
		_, ok := z.SetString(x, 10)
		if !ok {
			return nil, fmt.Errorf("invalid integer string: %s", x)
		}
		return z, nil
	default:
		return nil, fmt.Errorf("unsupported numeric type: %T", v)
	}
}

func leftPad64(hexNoPrefix string) string {
	hexNoPrefix = strings.TrimPrefix(strings.ToLower(hexNoPrefix), "0x")
	if hexNoPrefix == "" {
		hexNoPrefix = "0"
	}
	if len(hexNoPrefix) > 64 {
		return hexNoPrefix[len(hexNoPrefix)-64:]
	}
	return strings.Repeat("0", 64-len(hexNoPrefix)) + hexNoPrefix
}

func tronAddressToABIHex(addr string) (string, error) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return "", errors.New("empty address")
	}

	if strings.HasPrefix(addr, "T") {
		raw, err := base58Decode(addr)
		if err != nil {
			return "", fmt.Errorf("decode base58 address: %w", err)
		}
		if len(raw) != 25 {
			return "", fmt.Errorf("invalid base58 tron address length: %d", len(raw))
		}

		payload := raw[:21]
		if payload[0] != 0x41 {
			return "", errors.New("invalid tron address prefix")
		}

		return hex.EncodeToString(payload[1:]), nil
	}

	addr = strings.TrimPrefix(addr, "0x")
	addr = strings.TrimPrefix(addr, "0X")
	addr = strings.ToLower(addr)

	switch len(addr) {
	case 42:
		if !strings.HasPrefix(addr, "41") {
			return "", errors.New("expected hex tron address with 41 prefix")
		}
		return addr[2:], nil
	case 40:
		return addr, nil
	default:
		return "", fmt.Errorf("invalid address length: %d", len(addr))
	}
}
