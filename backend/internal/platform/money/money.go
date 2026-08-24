package money

import (
	"fmt"

	"github.com/shopspring/decimal"
)

type Amount = decimal.Decimal

func Parse(s string) (Amount, error) {
	d, err := decimal.NewFromString(s)
	if err != nil {
		return decimal.Zero, fmt.Errorf("invalid amount")
	}
	if d.IsNegative() {
		return decimal.Zero, fmt.Errorf("amount must be >= 0")
	}
	return d, nil
}

func Must(s string) Amount {
	d, err := Parse(s)
	if err != nil {
		panic(err)
	}
	return d
}

func Zero() Amount { return decimal.Zero }

func Round(a Amount, minorUnits int32) Amount {
	return a.RoundBank(minorUnits)
}
