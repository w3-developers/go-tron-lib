package main

import (
	"context"
	"log"
	"math/big"

	"github.com/snakoner/go-tron-lib"
)

const (
	fromAddress   = "TZJ32TTQgjqcYWQf626xTWaZUT9iKLXxtS"
	toAddress     = "TDxyML69uweBFRfoEBEbGYQUE3XTWzUPe8"
	fromAddresspk = "30aa9a4134118c36f4d458004697ae1c3f97680ac5fadfd560d84c6482ad04c6"
	trc20Address  = "TRPXG8YEMEaYE9dRs6fXvofFTiyMFE2mEg"
	rpc           = "https://nile.trongrid.io"
)

func main() {
	client := tron.NewSolid(rpc)
	trc20 := client.NewTRC20(trc20Address)

	balance, err := trc20.BalanceOf(context.Background(), fromAddress)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("balance: %s", balance)

	tx, err := trc20.BuildTransferTx(context.Background(), fromAddress, toAddress, big.NewInt(1000000), 100000000)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("tx: %s", tx)

	signedTx, err := tron.SignTransaction(tx, fromAddresspk)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("signedTx: %s", signedTx)

	resp, err := client.BroadcastTransaction(context.Background(), signedTx)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("resp: %s", resp)

	status, err := client.WaitForStatusSuccess(context.Background(), resp.TxID)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("status: %s", status)
}
