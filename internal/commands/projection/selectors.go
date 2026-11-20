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
	alias       string
}

func (selector columnSelector) String() string {
	label := selector.first.String()
	if selector.first != selector.last {
		label += "-" + selector.last.String()
	}
	if selector.alias != "" {
		label += "=" + selector.alias
	}
	return label
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
		for _, token := range splitColumnList(fs.Arg(0)) {
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
		selectors = append(selectors, columnSelector{first: ref, last: ref})
	}
	return selectors, nil
}

func parseCompactSelector(token string) (columnSelector, error) {
	source, alias, renamed := splitColumnAlias(token)
	source = decodeColumnEscapes(strings.TrimSpace(source))
	selector, err := parseColumnSource(source)
	if err != nil {
		return columnSelector{}, err
	}
	if renamed {
		alias = decodeColumnEscapes(strings.TrimSpace(alias))
		if alias == "" {
			return columnSelector{}, errors.New("output column name cannot be empty")
		}
		if selector.first != selector.last {
			return columnSelector{}, errors.New("rename one column at a time; a range cannot have one output name")
		}
		selector.alias = alias
	}
	return selector, nil
}

func parseColumnSource(token string) (columnSelector, error) {
	if token == "" {
		return columnSelector{}, errors.New("empty column selector")
	}
	if token[0] != '#' {
		if token[0] != '%' {
			token = "%" + token
		}
		ref, err := columns.Parse(token)
		return columnSelector{first: ref, last: ref}, err
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
	return columnSelector{first: first, last: last}, nil
}

// Backslashes escape only delimiters and backslashes. Other sequences retain
// their literal spelling, so names such as C:\Users keep working.
func isColumnEscape(value string, index int) bool {
	return value[index] == '\\' && index+1 < len(value) && strings.ContainsRune(",=\\", rune(value[index+1]))
}

func splitColumnList(value string) []string {
	var entries []string
	start := 0
	for i := 0; i < len(value); i++ {
		if isColumnEscape(value, i) {
			i++
		} else if value[i] == ',' {
			entries = append(entries, value[start:i])
			start = i + 1
		}
	}
	return append(entries, value[start:])
}

func splitColumnAlias(value string) (source, alias string, renamed bool) {
	for i := 0; i < len(value); i++ {
		if isColumnEscape(value, i) {
			i++
		} else if value[i] == '=' {
			return value[:i], value[i+1:], true
		}
	}
	return value, "", false
}

func decodeColumnEscapes(value string) string {
	var decoded strings.Builder
	for i := 0; i < len(value); i++ {
		if isColumnEscape(value, i) {
			i++
		}
		decoded.WriteByte(value[i])
	}
	return decoded.String()
}
