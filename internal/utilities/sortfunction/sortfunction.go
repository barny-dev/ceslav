package sortfunction

import (
	"cmp"
	"fmt"
	"math/big"

	"github.com/barny-dev/ceslav/internal/utilities/row"
)

type Ord = int

const OrdLE Ord = -1
const OrdEQ Ord = 0
const OrdGE Ord = 1

type SortFunction = func(a, b row.Row) Ord

func All(funcs ...SortFunction) SortFunction {
	return func(a, b row.Row) Ord {
		for _, f := range funcs {
			if ord := f(a, b); ord != OrdEQ {
				return ord
			}
		}
		return OrdEQ
	}
}

func AsString(ascending bool, columnIndex int) SortFunction {
	return func(a, b row.Row) Ord {
		v1 := a.Columns[columnIndex]
		v2 := b.Columns[columnIndex]
		o := cmp.Compare(v1, v2)
		if !ascending {
			o = -o
		}
		return o
	}
}

func AsDecimal(ascending bool, columnIndex int) SortFunction {
	return func(a, b row.Row) Ord {
		// Rows are validated before entering the heap.
		v1, _ := ParseDecimal(a.Columns[columnIndex])
		v2, _ := ParseDecimal(b.Columns[columnIndex])
		o := v1.Cmp(v2)
		if !ascending {
			o = -o
		}
		return o
	}
}

// ParseDecimal preserves the existing exact rational input grammar.
func ParseDecimal(text string) (*big.Rat, error) {
	value, ok := new(big.Rat).SetString(text)
	if !ok {
		return nil, fmt.Errorf("invalid number %q", text)
	}
	return value, nil
}
