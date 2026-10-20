package sqlite

import "math/big"

type scaledAmountTotal struct {
	scaled big.Int
}

func (t *scaledAmountTotal) add(value int64) {
	t.scaled.Add(&t.scaled, big.NewInt(value))
}

func (t *scaledAmountTotal) float64() float64 {
	value, _ := new(big.Rat).SetFrac(&t.scaled, big.NewInt(amountScale)).Float64()
	return value
}
