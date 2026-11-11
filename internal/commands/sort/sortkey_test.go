package sort

import "testing"

func TestParseSortKey(t *testing.T) {
	for _, tc := range []struct {
		source    string
		ascending bool
		valueType string
		header    []string
		index     int
	}{
		{"+s#0", true, ColumnTypeString, nil, 0},
		{"-d#12", false, ColumnTypeDecimal, nil, 12},
		{"-s%żółć", false, ColumnTypeString, []string{"name", "żółć"}, 1},
		{"+d%unit price", true, ColumnTypeDecimal, []string{"unit price"}, 0},
	} {
		t.Run(tc.source, func(t *testing.T) {
			key, err := ParseSortKey(tc.source)
			if err != nil {
				t.Fatal(err)
			}
			index, err := key.column.Resolve(tc.header)
			if err != nil {
				t.Fatal(err)
			}
			if key.sortDirection != tc.ascending || key.columnType != tc.valueType || index != tc.index {
				t.Fatalf("wrong parsed key: %+v, resolved index %d", key, index)
			}
		})
	}
}

func TestParseSortKeyRejectsInvalidSyntax(t *testing.T) {
	for _, source := range []string{"", "+", "+s", "+s#", "+s%", "xs#0", "+t#0", "+s0", "+s#-1", "+d#1.5", "+d#999999999999999999999999999"} {
		t.Run(source, func(t *testing.T) {
			if _, err := ParseSortKey(source); err == nil {
				t.Fatal("accepted invalid sort key")
			}
		})
	}
}
