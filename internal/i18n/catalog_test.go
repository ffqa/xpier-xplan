package i18n_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/ffqa/xpier-xplan/internal/config"
	"github.com/ffqa/xpier-xplan/internal/console"
	"github.com/ffqa/xpier-xplan/internal/i18n"
	"github.com/ffqa/xpier-xplan/internal/review"
)

// TestChineseCatalogCoversSource fails when a literal passed to i18n.T has no
// Chinese entry, so new messages cannot ship untranslated by accident.
func TestChineseCatalogCoversSource(t *testing.T) {
	catalog := i18n.Catalogs["zh"]
	for _, key := range sourceKeys(t, filepath.Join("..", "..")) {
		if _, ok := catalog[key]; !ok {
			t.Errorf("zh catalog is missing %q", key)
		}
	}
}

// TestChineseCatalogCoversTables checks the strings localized indirectly:
// review kinds, console commands, reviewer presets and templates.
func TestChineseCatalogCoversTables(t *testing.T) {
	catalog := i18n.Catalogs["zh"]
	expect := func(key string) {
		if _, ok := catalog[key]; !ok {
			t.Errorf("zh catalog is missing %q", key)
		}
	}
	for _, kind := range review.Kinds {
		expect(kind.Title)
		expect(kind.Description)
	}
	for _, command := range console.Commands {
		expect(command.Summary)
	}
	for _, preset := range config.BuiltinReviewers() {
		expect(preset.Description)
	}
	for _, template := range config.ReviewerTemplates {
		expect(template.Description)
		expect(template.Note)
	}
}

// TestChineseCatalogKeepsVerbs guards against translations that drop or
// reorder fmt verbs, which would garble the message at runtime.
func TestChineseCatalogKeepsVerbs(t *testing.T) {
	for key, translated := range i18n.Catalogs["zh"] {
		if want, got := verbs(key), verbs(translated); want != got {
			t.Errorf("verbs differ for %q: want %q, got %q", key, want, got)
		}
	}
}

func TestTranslateFallsBackToEnglish(t *testing.T) {
	t.Cleanup(func() { i18n.SetLanguage("en") })

	i18n.SetLanguage("zh")
	if got := i18n.T("failed"); got != "失败" {
		t.Fatalf("zh failed = %q", got)
	}
	if got := i18n.T("Status set to %s.", "done"); got != "状态已设为 done。" {
		t.Fatalf("zh with args = %q", got)
	}
	if got := i18n.T("no such key %d", 3); got != "no such key 3" {
		t.Fatalf("fallback = %q", got)
	}

	i18n.SetLanguage("fr")
	if got := i18n.Language(); got != "zh" {
		t.Fatalf("unknown language should be ignored, got %q", got)
	}
}

// sourceKeys returns every string literal passed as the first argument of
// i18n.T in the Go files under root.
func sourceKeys(t *testing.T, root string) []string {
	t.Helper()
	var keys []string
	fileSet := token.NewFileSet()
	walkErr := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if name := entry.Name(); name == "node_modules" || name == "web" || strings.HasPrefix(name, ".") && path != root {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, parseErr := parser.ParseFile(fileSet, path, nil, 0)
		if parseErr != nil {
			return parseErr
		}
		ast.Inspect(file, func(node ast.Node) bool {
			if key, ok := translateLiteral(node); ok {
				keys = append(keys, key)
			}
			return true
		})
		return nil
	})
	if walkErr != nil {
		t.Fatalf("walk source: %v", walkErr)
	}
	if len(keys) == 0 {
		t.Fatal("found no i18n.T calls; is the source root right?")
	}
	return keys
}

func translateLiteral(node ast.Node) (string, bool) {
	call, ok := node.(*ast.CallExpr)
	if !ok || len(call.Args) == 0 {
		return "", false
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "T" {
		return "", false
	}
	if pkg, ok := selector.X.(*ast.Ident); !ok || pkg.Name != "i18n" {
		return "", false
	}
	literal, ok := call.Args[0].(*ast.BasicLit)
	if !ok || literal.Kind != token.STRING {
		return "", false
	}
	value, err := strconv.Unquote(literal.Value)
	return value, err == nil
}

// verbs lists the fmt verbs in order, e.g. "%s%d".
func verbs(format string) string {
	var builder strings.Builder
	for index := 0; index < len(format)-1; index++ {
		if format[index] != '%' {
			continue
		}
		next := format[index+1]
		if next == '%' {
			index++
			continue
		}
		builder.WriteByte('%')
		builder.WriteByte(next)
		index++
	}
	return builder.String()
}
