package projection

import (
	"reflect"
	"strings"
	"testing"
)

func TestColumnAliasSyntax(t *testing.T) {
	for _, tc := range []struct {
		token, source, alias string
	}{
		{"name=Full name", "%name", "Full name"},
		{"#0=ID", "#0", "ID"},
		{"#0-0=ID", "#0", "ID"},
		{`a\=b=ratio\, value`, "%a=b", "ratio, value"},
		{`unit\,price=Unit price`, "%unit,price", "Unit price"},
		{`C:\Users=Path`, `%C:\Users`, "Path"},
		{`C:\\Users=Path`, `%C:\Users`, "Path"},
		{"name=label=value", "%name", "label=value"},
		{" name = Full name ", "%name", "Full name"},
	} {
		selector, err := parseCompactSelector(tc.token)
		if err != nil || selector.first.String() != tc.source || selector.alias != tc.alias {
			t.Errorf("%q: selector %v, alias %q, error %v", tc.token, selector, selector.alias, err)
		}
	}
	for _, token := range []string{"=Name", "name=", "name=  ", "#0-2=Name"} {
		if _, err := parseCompactSelector(token); err == nil {
			t.Errorf("accepted %q", token)
		}
	}
}

func TestRenamingConfigurationErrors(t *testing.T) {
	for _, tc := range []struct {
		drop bool
		args []string
		want string
	}{
		{false, []string{"#0=ID"}, "requires --header and --output-header"},
		{false, []string{"-j", "name=Person"}, "requires --header and --output-header"},
		{true, []string{"-j", "-k", "name=Person"}, "select only"},
	} {
		cmd := SelectCmd()
		if tc.drop {
			cmd = DropCmd()
		}
		if err := cmd.ParseFlags(tc.args); err != nil {
			t.Fatal(err)
		}
		_, err := NewProcessor(cmd, func([]string) error { t.Fatal("emitted invalid configuration"); return nil }, tc.drop)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%v: got %v, want %s", tc.args, err, tc.want)
		}
	}
}

func TestRenamingDoesNotModifyInputHeader(t *testing.T) {
	header := []string{"name", "city"}
	proc := newProjection(t, false, []string{"-j", "-k", "name=Person,city=Location"}, func([]string) error { return nil })
	if _, err := proc.ProcessHeader(header); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(header, []string{"name", "city"}) {
		t.Fatalf("input header changed: %v", header)
	}
}
