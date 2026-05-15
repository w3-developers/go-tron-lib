package tron

import (
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"golang.org/x/crypto/sha3"
)

func FunctionSelector(signature string) string {
	h := sha3.NewLegacyKeccak256()
	h.Write([]byte(signature))
	sum := h.Sum(nil)
	return hex.EncodeToString(sum[:4])
}

func ABIEncodeAddressParam(addr string) (string, error) {
	addr = strings.TrimSpace(addr)
	var hexAddr string
	var err error

	if strings.HasPrefix(addr, "T") {
		hexAddr, err = TronBase58ToHex(addr)
		if err != nil {
			return "", err
		}
	} else {
		hexAddr = strings.TrimPrefix(strings.ToLower(strings.TrimPrefix(addr, "0x")), "0x")
	}

	hexAddr = strings.TrimPrefix(hexAddr, "0x")
	hexAddr = strings.ToLower(hexAddr)

	b, err := hex.DecodeString(hexAddr)
	if err != nil {
		return "", err
	}
	if len(b) != 21 {
		return "", fmt.Errorf("tron address must be 21 bytes (41 + 20), got %d", len(b))
	}

	evm20 := b[1:]
	return leftPad32Hex(hex.EncodeToString(evm20)), nil
}

func ABIEncodeUint256Param(n *big.Int) (string, error) {
	if n == nil || n.Sign() < 0 {
		return "", errors.New("uint256 must be non-negative")
	}
	return leftPad32Hex(fmt.Sprintf("%x", n)), nil
}

func ABIEncodeStringParam(s string) (string, error) {
	head := leftPad32Hex("20")
	data := []byte(s)
	length := leftPad32Hex(fmt.Sprintf("%x", len(data)))
	dataHex := hex.EncodeToString(data)
	padLen := (32 - (len(data) % 32)) % 32
	if padLen > 0 {
		dataHex += strings.Repeat("00", padLen)
	}
	return head + length + dataHex, nil
}

func ABIConcatParams(words ...string) (string, error) {
	var b strings.Builder
	for _, w := range words {
		w = strings.TrimPrefix(w, "0x")
		if len(w)%2 != 0 {
			return "", errors.New("param hex must have even length")
		}
		b.WriteString(w)
	}
	return b.String(), nil
}

func leftPad32Hex(h string) string {
	h = strings.TrimPrefix(h, "0x")
	h = strings.ToLower(h)
	if len(h) > 64 {
		panic("hex longer than 32 bytes")
	}
	return strings.Repeat("0", 64-len(h)) + h
}
