package main

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"sync"

	"github.com/snakoner/go-tron-lib"
)

const (
	multicallAddress = "TPP1ToFfmVXVTeWfJHAjmXsWnUGV8EkmnW"
	rpc              = "https://nile.trongrid.io"
	tokenAddress     = "TRPXG8YEMEaYE9dRs6fXvofFTiyMFE2mEg"
)

var addresses = []string{
	"TFbBApWL6TfyBhB8Tr322NeSUPePjnH4qe",
	"TZJ32TTQgjqcYWQf626xTWaZUT9iKLXxtS",
}

func main() {
	wg := sync.WaitGroup{}
	results := make(chan []*big.Int)

	client := tron.New(rpc)
	multicall := client.NewMulticall(multicallAddress)
	balances, err := multicall.BalanceAt(context.Background(), addresses)
	if err != nil {
		log.Fatal(err)
	}

	for i, balance := range balances {
		fmt.Printf("balance[%s]: %s\n", addresses[i], balance)
	}

	return

	wg.Add(1)
	go func() {
		defer wg.Done()
		client := tron.New(rpc)
		multicall := client.NewMulticall(multicallAddress)

		balances, err := multicall.BalanceOf(context.Background(), tokenAddress, addresses)
		if err != nil {
			log.Fatal(err)
		}

		for i, balance := range balances {
			fmt.Printf("balance[%s]: %s\n", addresses[i], balance)
		}

		results <- balances

		fmt.Println("multicall done")
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		clientSolid := tron.NewSolid(rpc)
		multicallSolid := clientSolid.NewMulticall(multicallAddress)

		balancesSolid, err := multicallSolid.BalanceOf(context.Background(), tokenAddress, addresses)
		if err != nil {
			log.Fatal(err)
		}

		for i, balance := range balancesSolid {
			fmt.Printf("balance[%s]: %s\n", addresses[i], balance)
		}

		results <- balancesSolid

		fmt.Println("multicall done")
	}()

	wg.Wait()
	close(results)

	for result := range results {
		for i, balance := range result {
			fmt.Printf("balance[%s]: %s\n", addresses[i], balance)
		}
	}
}
