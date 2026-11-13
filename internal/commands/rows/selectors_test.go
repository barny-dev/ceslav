package rows

import (
	"errors"
	"reflect"
	"testing"
)

func TestSelectorsMatchReferenceOrdering(t *testing.T) {
	for _, tc := range []struct {
		spec     string
		expanded []int64
	}{
		{"1,3-4,7", []int64{1, 3, 4, 7}},
		{"7,4-3,1", []int64{7, 4, 3, 1}},
		{"3-1,2-4", []int64{3, 2, 1, 2, 3, 4}},
		{"2,2,1-3", []int64{2, 2, 1, 2, 3}},
		{"8-8,1", []int64{8, 1}},
		{" 1 , 03 - 4 , 7 ", []int64{1, 3, 4, 7}},
	} {
		t.Run(tc.spec, func(t *testing.T) {
			selection, err := parseRowSelection(tc.spec)
			if err != nil {
				t.Fatal(err)
			}
			for total := int64(0); total <= 8; total++ {
				for _, reverse := range []bool{false, true} {
					var want, got []int64
					for _, index := range tc.expanded {
						if index <= total {
							want = append(want, index)
						}
					}
					if reverse {
						for i, j := 0, len(want)-1; i < j; i, j = i+1, j-1 {
							want[i], want[j] = want[j], want[i]
						}
					}
					err := selection.forEach(total, reverse, func(index int64) error { got = append(got, index); return nil })
					if err != nil || !reflect.DeepEqual(got, want) {
						t.Fatalf("total=%d reverse=%v: got %v, want %v; error %v", total, reverse, got, want, err)
					}
				}
			}
			for index := int64(0); index <= 9; index++ {
				want := false
				for _, selected := range tc.expanded {
					want = want || selected == index
				}
				if got := selection.contains(index); got != want {
					t.Fatalf("contains(%d) = %v, want %v", index, got, want)
				}
			}
		})
	}
}

func TestSelectorSyntaxErrors(t *testing.T) {
	for _, spec := range []string{"", " ", "0", "-1", "+1", "1,", ",1", "1,,2", "1-", "-2", "1--2", "1-2-3", "1-0", "a", "1.5", "1 2", "9223372036854775808", "1-9223372036854775808"} {
		if _, err := parseRowSelection(spec); err == nil {
			t.Errorf("accepted %q", spec)
		}
	}
}

func TestHugeRangesAreClippedBeforeIteration(t *testing.T) {
	for _, tc := range []struct {
		spec string
		want []int64
	}{
		{"1-9223372036854775807", []int64{1, 2, 3}},
		{"9223372036854775807-1", []int64{3, 2, 1}},
		{"9223372036854775807", nil},
	} {
		selection, err := parseRowSelection(tc.spec)
		if err != nil {
			t.Fatal(err)
		}
		if len(selection.ranges) != 1 || len(selection.intervals) != 1 {
			t.Fatal("range expanded before input")
		}
		var got []int64
		err = selection.forEach(3, false, func(index int64) error { got = append(got, index); return nil })
		if err != nil || !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("%s: got %v, want %v; error %v", tc.spec, got, tc.want, err)
		}
	}
	const limit = int64(9223372036854775807)
	selection, err := parseRowSelection("9223372036854775807")
	if err != nil {
		t.Fatal(err)
	}
	visits := 0
	if err := selection.forEach(limit, false, func(index int64) error {
		visits++
		if index != limit {
			t.Fatalf("wrong index: %d", index)
		}
		return nil
	}); err != nil || visits != 1 || !selection.contains(limit) {
		t.Fatalf("integer limit: %d visits, error %v", visits, err)
	}
}

func TestSelectorIterationStopsOnVisitorError(t *testing.T) {
	selection, err := parseRowSelection("1-3,1")
	if err != nil {
		t.Fatal(err)
	}
	failure := errors.New("visitor failed")
	for _, reverse := range []bool{false, true} {
		calls := 0
		err := selection.forEach(3, reverse, func(int64) error { calls++; return failure })
		if !errors.Is(err, failure) || calls != 1 {
			t.Fatalf("calls %d, error %v", calls, err)
		}
	}
}
