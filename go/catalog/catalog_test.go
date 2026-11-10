package catalog

import (
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
	"strconv"
	"testing"
)

var channelName = regexp.MustCompile(`^[a-z][a-z_]*(\.[a-z_]+)+$`)

func declaredChannels(t *testing.T) map[string]string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), "catalog.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	declared := map[string]string{}
	ast.Inspect(file, func(n ast.Node) bool {
		spec, ok := n.(*ast.ValueSpec)
		if !ok || spec.Type == nil {
			return true
		}
		if ident, ok := spec.Type.(*ast.Ident); !ok || ident.Name != "Channel" {
			return true
		}
		for i, name := range spec.Names {
			value, _ := strconv.Unquote(spec.Values[i].(*ast.BasicLit).Value)
			declared[value] = name.Name
		}
		return true
	})
	return declared
}

func TestAllCoversEveryConstant(t *testing.T) {
	declared := declaredChannels(t)
	listed := map[Channel]bool{}
	for _, c := range All() {
		if listed[c] {
			t.Errorf("%q appears twice in All()", c)
		}
		listed[c] = true
	}
	for value, name := range declared {
		if !listed[Channel(value)] {
			t.Errorf("%s (%q) is missing from All()", name, value)
		}
	}
	if len(listed) != len(declared) {
		t.Errorf("All() has %d channels, catalog.go declares %d", len(listed), len(declared))
	}
}

func TestChannelNames(t *testing.T) {
	for _, c := range All() {
		if !channelName.MatchString(string(c)) {
			t.Errorf("%q does not match %s", c, channelName)
		}
	}
}
