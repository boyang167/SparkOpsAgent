package install

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"regexp"
	"sort"
	"strconv"
)

var pluginIDPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{1,62}$`)

func RewriteRegisterGo(existing []byte, modulePath string, pluginIDs []string) ([]byte, error) {
	if modulePath == "" {
		return nil, fmt.Errorf("module path is required")
	}
	if len(bytes.TrimSpace(existing)) > 0 {
		if _, err := parser.ParseFile(token.NewFileSet(), "register.go", existing, parser.ImportsOnly); err != nil {
			return nil, err
		}
	}

	ids, err := sortedPluginIDs(pluginIDs)
	if err != nil {
		return nil, err
	}

	file := &ast.File{Name: ast.NewIdent("plugins")}
	if len(ids) > 0 {
		importDecl := &ast.GenDecl{Tok: token.IMPORT}
		for _, id := range ids {
			importDecl.Specs = append(importDecl.Specs, &ast.ImportSpec{
				Name: ast.NewIdent("_"),
				Path: &ast.BasicLit{
					Kind:  token.STRING,
					Value: strconv.Quote(modulePath + "/internal/plugins/" + id),
				},
			})
		}
		file.Decls = append(file.Decls, importDecl)
	}

	var out bytes.Buffer
	if err := format.Node(&out, token.NewFileSet(), file); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func sortedPluginIDs(pluginIDs []string) ([]string, error) {
	seen := make(map[string]struct{}, len(pluginIDs))
	for _, id := range pluginIDs {
		if !pluginIDPattern.MatchString(id) {
			return nil, fmt.Errorf("unsafe plugin id: %q", id)
		}
		seen[id] = struct{}{}
	}

	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids, nil
}
