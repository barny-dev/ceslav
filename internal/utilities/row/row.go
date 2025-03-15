package row

import (
	"slices"
)

type Row struct {
	Columns []string
}

func InitRow(record []string) Row {
	return Row{
		Columns: slices.Clone(record),
	}
}
