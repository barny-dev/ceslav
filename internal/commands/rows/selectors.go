package rows

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type rowRange struct {
	first, last int64
}

type rowSelector struct {
	rowRange
	copies int64
}

type rowSelection struct {
	ranges    []rowSelector // Original order and direction, with compact repetition counts.
	intervals []rowRange    // Sorted, disjoint ascending intervals for membership.
	last      int64
}

func parseRowSelection(spec string) (*rowSelection, error) {
	selection := &rowSelection{}
	for _, token := range strings.Split(spec, ",") {
		selector, err := parseRowSelector(token)
		if err != nil {
			return nil, fmt.Errorf("invalid row selector %q: %w", token, err)
		}
		selection.ranges = append(selection.ranges, selector)
		first, last := selector.first, selector.last
		if first > last {
			first, last = last, first
		}
		selection.intervals = append(selection.intervals, rowRange{first, last})
		if last > selection.last {
			selection.last = last
		}
	}
	// Keep ranges compact: even 1-9223372036854775807 needs only one interval.
	sort.Slice(selection.intervals, func(i, j int) bool {
		return selection.intervals[i].first < selection.intervals[j].first
	})
	merged := selection.intervals[:0]
	for _, interval := range selection.intervals {
		if len(merged) == 0 || interval.first > merged[len(merged)-1].last {
			merged = append(merged, interval)
		} else if interval.last > merged[len(merged)-1].last {
			merged[len(merged)-1].last = interval.last
		}
	}
	selection.intervals = merged
	return selection, nil
}

func parseRowSelector(token string) (rowSelector, error) {
	repetition := strings.Split(token, "*")
	if len(repetition) > 2 {
		return rowSelector{}, errors.New("expected a row or range optionally followed by *COUNT")
	}
	copies := int64(1)
	if len(repetition) == 2 {
		var err error
		copies, err = parsePositiveDecimal(repetition[1])
		if err != nil {
			return rowSelector{}, fmt.Errorf("repetition count: %w", err)
		}
	}
	parts := strings.Split(repetition[0], "-")
	if len(parts) > 2 {
		return rowSelector{}, errors.New("expected a number or inclusive range")
	}
	first, err := parsePositiveDecimal(parts[0])
	if err != nil {
		return rowSelector{}, fmt.Errorf("row number: %w", err)
	}
	last := first
	if len(parts) == 2 {
		last, err = parsePositiveDecimal(parts[1])
		if err != nil {
			return rowSelector{}, fmt.Errorf("row number: %w", err)
		}
	}
	return rowSelector{rowRange{first, last}, copies}, nil
}

func parsePositiveDecimal(value string) (int64, error) {
	value = strings.TrimSpace(value)
	for _, digit := range value {
		if digit < '0' || digit > '9' {
			return 0, errors.New("expected a positive 64-bit decimal integer")
		}
	}
	number, err := strconv.ParseInt(value, 10, 64)
	if err != nil || number <= 0 {
		return 0, errors.New("expected a positive 64-bit decimal integer")
	}
	return number, nil
}

func (selection *rowSelection) contains(index int64) bool {
	i := sort.Search(len(selection.intervals), func(i int) bool {
		return selection.intervals[i].last >= index
	})
	return i < len(selection.intervals) && selection.intervals[i].first <= index
}

func (selection *rowSelection) inInputOrder() bool {
	var previousEnd int64
	for _, selector := range selection.ranges {
		if selector.first <= previousEnd || selector.first > selector.last ||
			selector.copies > 1 && selector.first != selector.last {
			return false
		}
		previousEnd = selector.last
	}
	return true
}

// Clip each range to observed input before iterating, so absent rows never
// consume memory or time proportional to a requested range's upper bound.
func (selection *rowSelection) forEach(total int64, reverse bool, visit func(int64) error) error {
	for i := 0; i < len(selection.ranges); i++ {
		span := selection.ranges[i]
		if reverse {
			span = selection.ranges[len(selection.ranges)-1-i]
			span.first, span.last = span.last, span.first
		}
		step := int64(1)
		if span.first <= span.last {
			span.last = min(span.last, total)
			if span.first > span.last {
				continue
			}
		} else {
			span.first = min(span.first, total)
			if span.first < span.last {
				continue
			}
			step = -1
		}
		for copy := int64(0); copy < span.copies; copy++ {
			for index := span.first; ; index += step {
				if err := visit(index); err != nil {
					return err
				}
				// Break before incrementing to avoid overflow at the integer limit.
				if index == span.last {
					break
				}
			}
		}
	}
	return nil
}
