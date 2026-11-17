package i18n

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestCatalogCompleteness(t *testing.T) {
	for id, m := range Catalog {
		if m.ZH == "" || m.EN == "" {
			t.Errorf("missing translation: %s", id)
		}
		if strings.Count(m.ZH, "%d") != strings.Count(m.EN, "%d") {
			t.Errorf("format mismatch: %s", id)
		}
		if Text("en", id) != m.EN || Text("zh-CN", id) != m.ZH {
			t.Errorf("wrong locale: %s", id)
		}
	}
	if got := fmt.Sprintf(Text("en", "changes"), 2); got != "2 changes" {
		t.Fatal(got)
	}
}
func TestUIKeysExist(t *testing.T) {
	file, e := parser.ParseFile(token.NewFileSet(), filepath.Join("..", "ui", "desktop.go"), nil, 0)
	if e != nil {
		t.Fatal(e)
	}
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) != 1 {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "text" {
			return true
		}
		literal, ok := call.Args[0].(*ast.BasicLit)
		if ok && literal.Kind == token.STRING {
			key, _ := strconv.Unquote(literal.Value)
			if _, ok := Catalog[key]; !ok {
				t.Errorf("missing UI translation: %s", key)
			}
		}
		return true
	})
}
