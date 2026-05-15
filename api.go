package tron

import (
	"context"
	"encoding/json"
	"math/big"
	"time"
)

const (
	TxStatusSuccess = "SUCCESS"
	TxStatusFailed  = "FAILED"
	TxStatusPending = "PENDING"
)

const (
	newBlockGenerationTime = 3 * time.Second
	maxSolidBlockWaitTime  = 80 * time.Second
)

type StatusFunc func(ctx context.Context, txID string) (string, error)
type Raw = json.RawMessage

func (c *Client) GetNowBlock(ctx context.Context) (Raw, error) {
	var out Raw
	err := c.Call(ctx, "getnowblock", nil, &out)
	return out, err
}

type GetBlockByNumReq struct {
	Num int64 `json:"num"`
}

func (c *Client) GetBlockByNum(ctx context.Context, num int64) (Raw, error) {
	var out Raw
	err := c.Call(ctx, "getblockbynum", GetBlockByNumReq{Num: num}, &out)
	return out, err
}

type GetBlockByIDReq struct {
	Value string `json:"value"`
}

func (c *Client) GetBlockByID(ctx context.Context, blockID string) (Raw, error) {
	var out Raw
	err := c.Call(ctx, "getblockbyid", GetBlockByIDReq{Value: blockID}, &out)
	return out, err
}

type GetTransactionByIDReq struct {
	Value string `json:"value"`
}

func (c *Client) GetTransactionByID(ctx context.Context, txID string) (Raw, error) {
	var out Raw
	err := c.Call(ctx, "gettransactionbyid", GetTransactionByIDReq{Value: txID}, &out)
	return out, err
}

type GetTransactionInfoByIDReq struct {
	Value string `json:"value"`
}

func (c *Client) GetTransactionInfoByID(ctx context.Context, txID string) (Raw, error) {
	var out Raw
	err := c.Call(ctx, "gettransactioninfobyid", GetTransactionInfoByIDReq{Value: txID}, &out)
	return out, err
}

type GetAccountReq struct {
	Address string `json:"address"`
	Visible bool   `json:"visible,omitempty"`
}

func (c *Client) GetAccount(ctx context.Context, address string) (Raw, error) {
	var out Raw
	err := c.Call(ctx, "getaccount", GetAccountReq{
		Address: address,
		Visible: c.visible,
	}, &out)
	return out, err
}

type TriggerConstantContractReq struct {
	OwnerAddress    string `json:"owner_address"`
	ContractAddress string `json:"contract_address"`
	Function        string `json:"function_selector"`
	Parameter       string `json:"parameter,omitempty"`
	CallValue       int64  `json:"call_value,omitempty"`
	FeeLimit        int64  `json:"fee_limit,omitempty"`
	Visible         bool   `json:"visible,omitempty"`
}

func (c *Client) TriggerConstantContract(ctx context.Context, req TriggerConstantContractReq) (Raw, error) {
	var out Raw
	err := c.Call(ctx, "triggerconstantcontract", req, &out)
	return out, err
}

type TriggerSmartContractReq struct {
	OwnerAddress    string `json:"owner_address"`
	ContractAddress string `json:"contract_address"`
	Function        string `json:"function_selector"`
	Parameter       string `json:"parameter,omitempty"`
	CallValue       int64  `json:"call_value,omitempty"`
	FeeLimit        int64  `json:"fee_limit,omitempty"`
	Visible         bool   `json:"visible,omitempty"`
}

func (c *Client) TriggerSmartContract(ctx context.Context, req TriggerSmartContractReq) (Raw, error) {
	var out Raw
	err := c.Call(ctx, "triggersmartcontract", req, &out)
	return out, err
}

func (c *Client) GetNodeInfo(ctx context.Context) (Raw, error) {
	var out Raw
	err := c.Call(ctx, "getnodeinfo", nil, &out)
	return out, err
}

type BroadcastResp struct {
	Result  bool   `json:"result"`
	TxID    string `json:"txid,omitempty"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

func (c *Client) BroadcastTransaction(ctx context.Context, signedTx []byte) (*BroadcastResp, error) {
	var out BroadcastResp
	if err := c.Call(ctx, "broadcasttransaction", json.RawMessage(signedTx), &out); err != nil {
		return nil, err
	}

	return &out, nil
}

type CreateTransactionReq struct {
	OwnerAddress string `json:"owner_address"`
	ToAddress    string `json:"to_address"`
	Amount       int64  `json:"amount"`
	Visible      bool   `json:"visible,omitempty"`
}

func (c *Client) CreateTransaction(ctx context.Context, req CreateTransactionReq) (Raw, error) {
	var out Raw
	err := c.Call(ctx, "createtransaction", req, &out)
	return out, err
}

func (c *Client) BuildTransferTRXTx(ctx context.Context, from string, to string, amount *big.Int) (Raw, error) {
	return c.CreateTransaction(ctx, CreateTransactionReq{
		OwnerAddress: from,
		ToAddress:    to,
		Amount:       amount.Int64(),
		Visible:      c.visible,
	})
}

type GetTransactionInfoStatusResult struct {
	BlockNumber int64 `json:"blockNumber"`
	Receipt     struct {
		Result string `json:"result"`
	} `json:"receipt"`
}

func (c *Client) GetTransactionStatus(ctx context.Context, txID string) (string, error) {
	tx, err := c.GetTransactionInfoByID(ctx, txID)
	if err != nil {
		return "", err
	}

	var out GetTransactionInfoStatusResult
	if err := json.Unmarshal(tx, &out); err != nil {
		return "", err
	}

	return convertTransactionStatus(out), nil
}

func convertTransactionStatus(result GetTransactionInfoStatusResult) string {
	if result.BlockNumber == 0 {
		return TxStatusPending
	}

	resultStr := result.Receipt.Result
	if resultStr == "" || resultStr == TxStatusSuccess {
		return TxStatusSuccess
	}

	return TxStatusFailed
}

func (c *Client) WaitForStatusSuccess(ctx context.Context, txID string, opts ...time.Duration) (string, error) {
	maxWaitTime := maxSolidBlockWaitTime
	if len(opts) > 0 {
		maxWaitTime = opts[0]
	}

	return c.waitForStatus(ctx, txID, maxWaitTime)
}

func (c *Client) waitForStatus(
	ctx context.Context,
	txID string,
	maxWaitTime time.Duration,
) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, maxWaitTime)
	defer cancel()

	ticker := time.NewTicker(newBlockGenerationTime)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return TxStatusFailed, ctx.Err()
		case <-ticker.C:
			status, err := c.GetTransactionStatus(ctx, txID)
			if err != nil {
				return "", err
			}
			switch status {
			case TxStatusSuccess,
				TxStatusFailed:
				return status, nil
			case TxStatusPending:
				continue
			}
		}
	}
}

type getAccountResp struct {
	Balance int64 `json:"balance"`
}

func (c *Client) BalanceAt(ctx context.Context, address string) (*big.Int, error) {
	raw, err := c.GetAccount(ctx, address)
	if err != nil {
		return nil, err
	}

	var resp getAccountResp
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, err
	}

	return big.NewInt(resp.Balance), nil
}

func (c *Client) BalanceOf(ctx context.Context, tokenAddress string, address string) (*big.Int, error) {
	return c.NewTRC20(tokenAddress).BalanceOf(ctx, address)
}

func (c *Client) TransferToken(ctx context.Context, tokenAddress string, to string, amount *big.Int, privateKey string) (string, error) {
	from, err := PrivateKeyHexToAddressBase58(privateKey)
	if err != nil {
		return "", err
	}

	trc20 := c.NewTRC20(tokenAddress)
	tx, err := trc20.BuildTransferTx(ctx, from, to, amount, 100000000)
	if err != nil {
		return "", err
	}

	signedTx, err := SignTransaction(tx, privateKey)
	if err != nil {
		return "", err
	}

	resp, err := c.BroadcastTransaction(ctx, signedTx)
	if err != nil {
		return "", err
	}

	return resp.TxID, nil
}

func (c *Client) TransferNative(ctx context.Context, to string, amount *big.Int, privateKey string) (string, error) {
	from, err := PrivateKeyHexToAddressBase58(privateKey)
	if err != nil {
		return "", err
	}

	tx, err := c.BuildTransferTRXTx(ctx, from, to, amount)
	if err != nil {
		return "", err
	}

	signedTx, err := SignTransaction(tx, privateKey)
	if err != nil {
		return "", err
	}

	resp, err := c.BroadcastTransaction(ctx, signedTx)
	if err != nil {
		return "", err
	}

	return resp.TxID, nil
}
