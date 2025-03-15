package sort

import (
	"errors"
	"fmt"
	"strconv"
)

const ColumnTypeString = "string"
const ColumnTypeDecimal = "decimal"

const ColumnByName = "name"
const ColumnByIndex = "index"

const DirectionAscending = true
const DirectionDescending = false

type SortKey struct {
	sortDirection bool
	columnType    string
	columnBy      string
	columnIndex   uint64
	columnName    string
}

func ParseSortKey(src string) (SortKey, error) {
	runes := []rune(src)

	if len(runes) < 4 {
		return SortKey{}, errors.New("sort key too short")
	}

	direction, err := parseDirection(runes[0])
	if err != nil {
		return SortKey{}, fmt.Errorf("sort-key: char [0] - %v", err)
	}

	columnType, err := parseType(runes[1])
	if err != nil {
		return SortKey{}, fmt.Errorf("sort-key: char [1] - %v", err)
	}

	columnBy, columnName, columnIndex, err := parseColumnSpecifier(runes[2:])
	if err != nil {
		return SortKey{}, fmt.Errorf("sort-key: char [2:%d] - %v", len(runes), err)
	}
	return SortKey{
		sortDirection: direction,
		columnType:    columnType,
		columnBy:      columnBy,
		columnIndex:   columnIndex,
		columnName:    columnName,
	}, nil
}

func parseDirection(ch rune) (bool, error) {
	if ch == '+' {
		return DirectionAscending, nil
	} else if ch == '-' {
		return DirectionDescending, nil
	} else {
		return false, errors.New("sort direction specifier must be [+|-]")
	}
}

func parseType(ch rune) (string, error) {
	if ch == 's' {
		return ColumnTypeString, nil
	} else if ch == 'd' {
		return ColumnTypeDecimal, nil
	} else {
		return "", errors.New("column type specifier must be [s|d]")
	}
}

func parseColumnSpecifier(runes []rune) (
	columnBy string,
	columnName string,
	columnIndex uint64,
	err error,
) {
	if runes[0] == '%' {
		columnBy = ColumnByName
		columnName = string(runes[1:])
	} else if runes[0] == '#' {
		columnBy = ColumnByIndex
		columnIndex, err = strconv.ParseUint(string(runes[1:]), 10, 32)
		if err != nil {
			err = errors.New("column index must be valid integer")
		}
	} else {
		err = errors.New("column specifier must start with [%|#]")
	}
	return columnBy, columnName, columnIndex, err
}
