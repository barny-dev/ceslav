// Package columns resolves CSV column references shared by commands.
package columns

import (
	"fmt"
	"strconv"
)

// Reference identifies a column by its zero-based index or exact header name.
type Reference struct {
	index  int
	name   string
	byName bool
}

func Parse(selector string) (Reference, error) {
	if len(selector) < 2 {
		return Reference{}, fmt.Errorf("column selector %q requires #index or %%name", selector)
	}
	switch selector[0] {
	case '%':
		return Reference{name: selector[1:], byName: true}, nil
	case '#':
		index, err := strconv.ParseUint(selector[1:], 10, strconv.IntSize-1)
		if err != nil {
			return Reference{}, fmt.Errorf("column index %q must be a nonnegative integer fitting in int", selector[1:])
		}
		return Reference{index: int(index)}, nil
	default:
		return Reference{}, fmt.Errorf("column selector %q must start with # or %%", selector)
	}
}

func (ref Reference) String() string {
	if ref.byName {
		return "%" + ref.name
	}
	return "#" + strconv.Itoa(ref.index)
}

// Resolve validates indices against a known header. Without a header, callers
// must validate the returned index against each record before accessing it.
func (ref Reference) Resolve(header []string) (int, error) {
	if !ref.byName {
		if header != nil {
			if err := ValidateIndex(ref.index, len(header)); err != nil {
				return 0, err
			}
		}
		return ref.index, nil
	}
	if header == nil {
		return 0, fmt.Errorf("column %%%s requires --header", ref.name)
	}
	found := -1
	for i, name := range header {
		if name == ref.name {
			if found >= 0 {
				return 0, fmt.Errorf("column %%%s is ambiguous: duplicate header name; use #index", ref.name)
			}
			found = i
		}
	}
	if found < 0 {
		return 0, fmt.Errorf("column %%%s not found in header", ref.name)
	}
	return found, nil
}

func ValidateIndex(index, width int) error {
	if index < 0 || index >= width {
		return fmt.Errorf("column index #%d out of range for %d columns (indices start at 0)", index, width)
	}
	return nil
}
