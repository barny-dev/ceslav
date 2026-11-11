package columns

import (
	"strings"
	"testing"
)

func TestResolve(t *testing.T) {
	cases := []struct {
		selector  string
		header    []string
		index     int
		errorPart string
	}{
		{"%amount", []string{"name", "amount"}, 1, ""},
		{"%cena brutto", []string{"cena brutto", "name"}, 0, ""},
		{"%żółć", []string{"name", "żółć"}, 1, ""},
		{"#0", []string{"same", "same"}, 0, ""},
		{"#2", nil, 2, ""},
		{"%amount", nil, 0, "requires --header"},
		{"%amount", []string{"name"}, 0, "not found"},
		{"%same", []string{"same", "same"}, 0, "ambiguous"},
		{"#2", []string{"one", "two"}, 0, "out of range"},
	}
	for _, tc := range cases {
		t.Run(tc.selector+"/"+tc.errorPart, func(t *testing.T) {
			ref, err := Parse(tc.selector)
			if err != nil {
				t.Fatal(err)
			}
			index, err := ref.Resolve(tc.header)
			if tc.errorPart != "" {
				if err == nil || !strings.Contains(err.Error(), tc.errorPart) {
					t.Fatalf("error: %v", err)
				}
			} else if err != nil || index != tc.index {
				t.Fatalf("index %d, error %v; want %d", index, err, tc.index)
			}
		})
	}
}

func TestInvalidSelectors(t *testing.T) {
	for _, selector := range []string{"", "#", "%", "amount", "#-1", "#1.5", "#99999999999999999999999999"} {
		t.Run(selector, func(t *testing.T) {
			if _, err := Parse(selector); err == nil {
				t.Fatal("accepted invalid selector")
			}
		})
	}
}
