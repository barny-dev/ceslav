package projection

import (
	"errors"
	"fmt"
	"strings"

	"github.com/barny-dev/ceslav/internal/columns"
	"github.com/spf13/cobra"
)

// A singleton has equal endpoints. Ranges remain compact until validated
// against the input width, preventing allocations based on untrusted indices.
type columnSelector struct {
	first, last columns.Reference
}

func (selector columnSelector) String() string {
	if selector.first == selector.last {
		return selector.first.String()
	}
	return selector.first.String() + "-" + selector.last.String()
}

func readSelectors(cmd *cobra.Command) ([]columnSelector, error) {
	fs := cmd.Flags()
	if fs.NArg() > 1 {
		return nil, errors.New("expected one comma-separated column list")
	}
	literals, err := fs.GetStringArray("column")
	if err != nil {
		return nil, err
	}
	if fs.NArg() == 1 {
		if fs.Changed("column") {
			return nil, errors.New("use either a column list or --column, not both")
		}
		var selectors []columnSelector
		for _, token := range strings.Split(fs.Arg(0), ",") {
			selector, err := parseCompactSelector(strings.TrimSpace(token))
			if err != nil {
				return nil, fmt.Errorf("column list entry %q: %w", token, err)
			}
			selectors = append(selectors, selector)
		}
		return selectors, nil
	}
	if len(literals) == 0 {
		return nil, errors.New("provide a comma-separated column list or at least one --column")
	}
	var selectors []columnSelector
	for _, literal := range literals {
		ref, err := columns.Parse(literal)
		if err != nil {
			return nil, err
		}
		selectors = append(selectors, columnSelector{ref, ref})
	}
	return selectors, nil
}

func parseCompactSelector(token string) (columnSelector, error) {
	if token == "" {
		return columnSelector{}, errors.New("empty column selector")
	}
	if token[0] != '#' {
		if token[0] != '%' {
			token = "%" + token
		}
		ref, err := columns.Parse(token)
		return columnSelector{ref, ref}, err
	}
	parts := strings.Split(token[1:], "-")
	if len(parts) > 2 {
		return columnSelector{}, errors.New("expected #index or inclusive #start-end range")
	}
	first, err := columns.Parse("#" + strings.TrimSpace(parts[0]))
	if err != nil {
		return columnSelector{}, err
	}
	last := first
	if len(parts) == 2 {
		last, err = columns.Parse("#" + strings.TrimPrefix(strings.TrimSpace(parts[1]), "#"))
		if err != nil {
			return columnSelector{}, err
		}
	}
	return columnSelector{first, last}, nil
}
