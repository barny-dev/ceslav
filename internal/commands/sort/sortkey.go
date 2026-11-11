package sort

import (
	"errors"
	"fmt"

	"github.com/barny-dev/ceslav/internal/columns"
)

const ColumnTypeString = "string"
const ColumnTypeDecimal = "decimal"
const DirectionAscending = true
const DirectionDescending = false

type SortKey struct {
	sortDirection bool
	columnType    string
	column        columns.Reference
}

func ParseSortKey(src string) (SortKey, error) {
	if len(src) < 4 {
		return SortKey{}, errors.New("sort key too short: expected direction, type and column (for example +s#0)")
	}
	direction, err := parseDirection(src[0])
	if err != nil {
		return SortKey{}, fmt.Errorf("sort-key %q: %w", src, err)
	}
	columnType, err := parseType(src[1])
	if err != nil {
		return SortKey{}, fmt.Errorf("sort-key %q: %w", src, err)
	}
	column, err := columns.Parse(src[2:])
	if err != nil {
		return SortKey{}, fmt.Errorf("sort-key %q: %w", src, err)
	}
	return SortKey{sortDirection: direction, columnType: columnType, column: column}, nil
}

func parseDirection(ch byte) (bool, error) {
	switch ch {
	case '+':
		return DirectionAscending, nil
	case '-':
		return DirectionDescending, nil
	default:
		return false, errors.New("sort direction specifier must be [+|-]")
	}
}

func parseType(ch byte) (string, error) {
	switch ch {
	case 's':
		return ColumnTypeString, nil
	case 'd':
		return ColumnTypeDecimal, nil
	default:
		return "", errors.New("column type specifier must be [s|d]")
	}
}
