package feature

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestUserFacingCopyAvoidsSentenceSeparators(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
	for _, directory := range []string{"cmd", "internal"} {
		err := filepath.WalkDir(filepath.Join(root, directory), func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || strings.HasSuffix(path, "_test.go") || strings.Contains(path, string(filepath.Separator)+"testdata"+string(filepath.Separator)) {
				return nil
			}
			switch filepath.Ext(path) {
			case ".go":
				checkGoCopy(t, root, path)
			case ".js":
				checkJSCopy(t, root, path)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func checkGoCopy(t *testing.T, root, path string) {
	t.Helper()
	set := token.NewFileSet()
	file, err := parser.ParseFile(set, path, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	ast.Inspect(file, func(node ast.Node) bool {
		literal, ok := node.(*ast.BasicLit)
		if !ok || literal.Kind != token.STRING {
			return true
		}
		value, err := strconv.Unquote(literal.Value)
		if err != nil || allowedCodeLiteral(value) {
			return true
		}
		checkCopy(t, root, path, set.Position(literal.Pos()).Line, value)
		return true
	})
}

func allowedCodeLiteral(value string) bool {
	return strings.Contains(value, "; charset=") ||
		strings.HasPrefix(value, "default-src ") ||
		strings.Contains(value, "<!doctype html>") ||
		strings.HasPrefix(value, "D:P(") ||
		strings.Contains(value, ";;FA;;;")
}

func checkJSCopy(t *testing.T, root, path string) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, literal := range jsStringLiterals(string(b)) {
		checkCopy(t, root, path, literal.line, literal.value)
	}
}

type sourceLiteral struct {
	line  int
	value string
}

// jsStringLiterals extracts authored text without treating JavaScript statement
// semicolons as copy. Module assets intentionally use no build-time parser.
func jsStringLiterals(source string) []sourceLiteral {
	var result []sourceLiteral
	line := 1
	for i := 0; i < len(source); {
		if source[i] == '\n' {
			line++
			i++
			continue
		}
		if i+1 < len(source) && source[i:i+2] == "//" {
			if end := strings.IndexByte(source[i+2:], '\n'); end >= 0 {
				i += end + 2
				continue
			}
			break
		}
		if i+1 < len(source) && source[i:i+2] == "/*" {
			end := strings.Index(source[i+2:], "*/")
			if end < 0 {
				break
			}
			segment := source[i : i+end+4]
			line += strings.Count(segment, "\n")
			i += end + 4
			continue
		}
		quote := source[i]
		if quote != '\'' && quote != '"' && quote != '`' {
			i++
			continue
		}
		startLine := line
		i++
		var value strings.Builder
		for i < len(source) {
			if source[i] == '\\' && i+1 < len(source) {
				value.WriteByte(source[i])
				value.WriteByte(source[i+1])
				i += 2
				continue
			}
			if source[i] == quote {
				i++
				break
			}
			if source[i] == '\n' {
				line++
			}
			value.WriteByte(source[i])
			i++
		}
		result = append(result, sourceLiteral{line: startLine, value: value.String()})
	}
	return result
}

func checkCopy(t *testing.T, root, path string, line int, value string) {
	t.Helper()
	if strings.ContainsAny(value, ";–—") {
		relative, err := filepath.Rel(root, path)
		if err != nil {
			relative = path
		}
		t.Errorf("%s:%d contains a disallowed sentence separator in %q", relative, line, value)
	}
}
