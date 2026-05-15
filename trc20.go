package tron

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/crypto"
)

const (
	ownerFromAddressStub = "TTyNBH7UDfxY1wqyjq9CsTgYM9p5KnNB3b"
)

type TRC20 struct {
	c        *Client
	contract string
}

func (c *Client) NewTRC20(contract string) *TRC20 {
	return &TRC20{c: c, contract: contract}
}

type TriggerConstResult struct {
	Result struct {
		Result bool `json:"result"`
	} `json:"result"`
	ConstantResult []string `json:"constant_result"`
	Message        string   `json:"message,omitempty"`
}

func (t *TRC20) BalanceOf(ctx context.Context, owner string) (*big.Int, error) {
	param, err := ABIEncodeAddressParam(owner)
	if err != nil {
		return nil, err
	}

	raw, err := t.c.TriggerConstantContract(ctx, TriggerConstantContractReq{
		OwnerAddress:    owner,
		ContractAddress: t.contract,
		Function:        "balanceOf(address)",
		Parameter:       param,
		Visible:         t.c.visible,
	})
	if err != nil {
		return nil, err
	}

	var out TriggerConstResult
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	if !out.Result.Result {
		if out.Message != "" {
			return nil, errors.New(out.Message)
		}
		return nil, errors.New("constant call failed")
	}
	if len(out.ConstantResult) == 0 {
		return nil, errors.New("empty constant_result")
	}
	return parseUint256Hex(out.ConstantResult[0])
}

func (t *TRC20) Decimals(ctx context.Context) (uint8, error) {
	v, err := t.callUint256NoArgs(ctx, "decimals()", ownerFromAddressStub)
	if err != nil {
		return 0, err
	}
	if v.Sign() < 0 || v.BitLen() > 8 {
		return 0, errors.New("decimals out of range")
	}
	return uint8(v.Uint64()), nil
}

func (t *TRC20) TotalSupply(ctx context.Context) (*big.Int, error) {
	return t.callUint256NoArgs(ctx, "totalSupply()", ownerFromAddressStub)
}

func (t *TRC20) Name(ctx context.Context) (string, error) {
	return t.callStringBestEffort(ctx, "name()", ownerFromAddressStub)
}

func (t *TRC20) Symbol(ctx context.Context) (string, error) {
	return t.callStringBestEffort(ctx, "symbol()", ownerFromAddressStub)
}

func (t *TRC20) callUint256NoArgs(ctx context.Context, fn string, ownerFrom string) (*big.Int, error) {
	raw, err := t.c.TriggerConstantContract(ctx, TriggerConstantContractReq{
		OwnerAddress:    ownerFrom,
		ContractAddress: t.contract,
		Function:        fn,
		Visible:         t.c.visible,
	})
	if err != nil {
		return nil, err
	}
	var out TriggerConstResult
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	if !out.Result.Result {
		if out.Message != "" {
			return nil, errors.New(out.Message)
		}
		return nil, errors.New("constant call failed")
	}
	if len(out.ConstantResult) == 0 {
		return nil, errors.New("empty constant_result")
	}
	return parseUint256Hex(out.ConstantResult[0])
}

func (t *TRC20) callStringBestEffort(ctx context.Context, fn string, ownerFrom string) (string, error) {
	raw, err := t.c.TriggerConstantContract(ctx, TriggerConstantContractReq{
		OwnerAddress:    ownerFrom,
		ContractAddress: t.contract,
		Function:        fn,
		Visible:         t.c.visible,
	})
	if err != nil {
		return "", err
	}

	var out TriggerConstResult
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", err
	}
	if !out.Result.Result {
		if out.Message != "" {
			return "", errors.New(out.Message)
		}
		return "", errors.New("constant call failed")
	}
	if len(out.ConstantResult) == 0 {
		return "", errors.New("empty constant_result")
	}

	hexRet := strings.TrimPrefix(out.ConstantResult[0], "0x")
	hexRet = strings.TrimPrefix(hexRet, "0X")
	hexRet = strings.TrimSpace(hexRet)

	b, err := hex.DecodeString(hexRet)
	if err != nil {
		return "", err
	}

	if len(b) >= 64 {
		off := new(big.Int).SetBytes(b[:32]).Int64()
		if off >= 0 && off+32 <= int64(len(b)) {
			l := new(big.Int).SetBytes(b[off : off+32]).Int64()
			start := off + 32
			end := start + l
			if l >= 0 && end <= int64(len(b)) {
				return string(b[start:end]), nil
			}
		}
	}

	trim := bytesTrimRightZero(b)
	return string(trim), nil
}

func bytesTrimRightZero(b []byte) []byte {
	i := len(b)
	for i > 0 && b[i-1] == 0 {
		i--
	}
	return b[:i]
}

func parseUint256Hex(h string) (*big.Int, error) {
	h = strings.TrimPrefix(strings.TrimSpace(h), "0x")
	b, err := hex.DecodeString(h)
	if err != nil {
		return nil, err
	}
	return new(big.Int).SetBytes(b), nil
}

type TriggerSmartResult struct {
	Result struct {
		Result bool `json:"result"`
	} `json:"result"`
	Transaction json.RawMessage `json:"transaction"`
	Message     string          `json:"message,omitempty"`
}

func (t *TRC20) BuildTransferTx(
	ctx context.Context,
	ownerFrom string,
	to string,
	amount *big.Int,
	feeLimit int64,
) (json.RawMessage, error) {
	if amount == nil || amount.Sign() < 0 {
		return nil, errors.New("amount must be non-negative")
	}

	toP, err := ABIEncodeAddressParam(to)
	if err != nil {
		return nil, err
	}
	amtP, err := ABIEncodeUint256Param(amount)
	if err != nil {
		return nil, err
	}
	param, err := ABIConcatParams(toP, amtP)
	if err != nil {
		return nil, err
	}

	raw, err := t.c.TriggerSmartContract(ctx, TriggerSmartContractReq{
		OwnerAddress:    ownerFrom,
		ContractAddress: t.contract,
		Function:        "transfer(address,uint256)",
		Parameter:       param,
		FeeLimit:        feeLimit,
		Visible:         t.c.visible,
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

type TronTx struct {
	Visible    bool            `json:"visible,omitempty"`
	TxID       string          `json:"txID,omitempty"`
	RawData    json.RawMessage `json:"raw_data,omitempty"`
	RawDataHex string          `json:"raw_data_hex"`
	Signature  []string        `json:"signature,omitempty"`
}

func SignTransaction(txJSON []byte, privateKeyHex string) ([]byte, error) {
	if len(txJSON) == 0 {
		return nil, errors.New("empty tx json")
	}
	privateKeyHex = strings.TrimSpace(privateKeyHex)
	privateKeyHex = strings.TrimPrefix(privateKeyHex, "0x")
	privateKeyHex = strings.TrimPrefix(privateKeyHex, "0X")
	if privateKeyHex == "" {
		return nil, errors.New("empty private key")
	}

	priv, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		return nil, fmt.Errorf("invalid private key: %w", err)
	}

	var tx TronTx
	if err := json.Unmarshal(txJSON, &tx); err != nil {
		return nil, fmt.Errorf("unmarshal tx: %w", err)
	}
	if tx.RawDataHex == "" {
		return nil, errors.New("missing raw_data_hex in tx")
	}

	rawBytes, err := hex.DecodeString(strings.TrimPrefix(tx.RawDataHex, "0x"))
	if err != nil {
		return nil, fmt.Errorf("decode raw_data_hex: %w", err)
	}

	h := sha256.Sum256(rawBytes)
	txidHex := hex.EncodeToString(h[:])

	if tx.TxID == "" {
		tx.TxID = txidHex
	} else if !strings.EqualFold(tx.TxID, txidHex) {
		return nil, fmt.Errorf("txID mismatch: json=%s computed=%s", tx.TxID, txidHex)
	}

	sig, err := crypto.Sign(h[:], priv)
	if err != nil {
		return nil, fmt.Errorf("sign failed: %w", err)
	}
	sigHex := hex.EncodeToString(sig)

	tx.Signature = append(tx.Signature, sigHex)

	out, err := json.Marshal(tx)
	if err != nil {
		return nil, fmt.Errorf("marshal signed tx: %w", err)
	}
	return out, nil
}
