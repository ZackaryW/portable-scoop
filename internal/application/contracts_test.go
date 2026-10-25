package application

import (
	"go/parser"
	"go/token"
	"strconv"
	"strings"
	"testing"
)

func TestApplicationBoundaryExcludesPackageDispatch(t *testing.T) {
	pkgs, err := parser.ParseDir(token.NewFileSet(), ".", nil, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	if len(pkgs) == 0 {
		t.Skip("source architecture check runs in the repository; standalone guest has no Go sources")
	}
	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			for _, imp := range file.Imports {
				path, _ := strconv.Unquote(imp.Path.Value)
				if strings.HasPrefix(path, "portscoop/internal/backend") || path == "portscoop/internal/cli" || path == "portscoop/internal/delegation" {
					t.Errorf("application imports package dispatch %s", path)
				}
			}
		}
	}
}
