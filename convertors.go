package tron

import "math/big"

const (
	TrxDecimals = 6
)

func BigIntToFloat64(n *big.Int) float64 {
	f, _ := n.Float64()
	return f
}

func Float64ToBigInt(f float64) *big.Int {
	return big.NewInt(int64(f))
}

func BigIntToBigFloat(n *big.Int) *big.Float {
	return big.NewFloat(0).SetInt(n)
}

func SunToTRX(sunValue *big.Int) *big.Float {
	sunBigFloat := BigIntToBigFloat(sunValue)

	base := big.NewInt(10)
	exp := big.NewInt(6)
	decimalsPow := new(big.Int).Exp(base, exp, nil)
	decimalsBigFloat := big.NewFloat(0).SetInt(decimalsPow)

	return sunBigFloat.Quo(sunBigFloat, decimalsBigFloat)
}

func TRXToSun(trxValue *big.Float) *big.Int {
	base := big.NewInt(10)
	exp := big.NewInt(6)
	decimalsPow := new(big.Int).Exp(base, exp, nil)
	decimalsBigFloat := big.NewFloat(0).SetInt(decimalsPow)

	trxBigFloat := big.NewFloat(0).Set(trxValue)

	res := trxBigFloat.Mul(trxBigFloat, decimalsBigFloat)
	resInt, _ := res.Int(nil)
	return resInt
}

func ToHumanAmount(amount *big.Int, decimals uint8) *big.Float {
	base := big.NewInt(10)
	exp := big.NewInt(int64(decimals))
	decimalsPow := new(big.Int).Exp(base, exp, nil)
	amountBigFloat := big.NewFloat(0).SetInt(amount)

	res := amountBigFloat.Quo(amountBigFloat, big.NewFloat(0).SetInt(decimalsPow))
	return res
}

func ToBlockchainAmount(amount *big.Float, decimals uint8) *big.Int {
	base := big.NewInt(10)
	exp := big.NewInt(int64(decimals))
	decimalsPow := new(big.Int).Exp(base, exp, nil)
	amountBigFloat := big.NewFloat(0).Set(amount)

	res := amountBigFloat.Mul(amountBigFloat, big.NewFloat(0).SetInt(decimalsPow))
	resInt, _ := res.Int(nil)
	return resInt
}
