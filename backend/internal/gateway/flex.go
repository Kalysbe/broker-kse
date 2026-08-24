package gateway

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/shopspring/decimal"
)

type Number string

func (n *Number) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		*n = ""
		return nil
	}
	if len(b) > 0 && b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		*n = Number(strings.TrimSpace(s))
		return nil
	}
	*n = Number(strings.TrimSpace(string(b)))
	return nil
}

func (n Number) Decimal() (decimal.Decimal, error) {
	s := strings.TrimSpace(string(n))
	if s == "" {
		return decimal.Zero, fmt.Errorf("empty number")
	}
	return decimal.NewFromString(s)
}

func (n Number) String() string { return string(n) }
