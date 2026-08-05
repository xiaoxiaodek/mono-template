package identity

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestBizIdentityImportBoundary(t *testing.T) {
	forbidden := []string{
		"/operation-api/internal/data/", "/operation-api/internal/service/", "/operation-api/internal/server",
		"/operation-api/internal/modules/", "/apps/internal/platform/security", "/apps/pkg/idgen",
		"github.com/gin-gonic/gin", "github.com/jackc/pgx/", "github.com/redis/go-redis/",
	}
	err := filepath.WalkDir(".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, parseErr := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if parseErr != nil {
			return parseErr
		}
		for _, spec := range file.Decls {
			declaration, ok := spec.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, item := range declaration.Specs {
				importSpec, ok := item.(*ast.ImportSpec)
				if !ok {
					continue
				}
				name, unquoteErr := strconv.Unquote(importSpec.Path.Value)
				if unquoteErr != nil {
					return unquoteErr
				}
				for _, fragment := range forbidden {
					if strings.Contains(name, fragment) {
						t.Errorf("%s imports forbidden dependency %q", path, name)
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
