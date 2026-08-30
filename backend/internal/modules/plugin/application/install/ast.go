package install

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
)

const sparkopsModulePrefix = "github.com/sparkops/sparkops-agent/backend/internal/"

const ginImportPath = "github.com/gin-gonic/gin"

func FileDeclaresRegisterEngine(src []byte) bool {
	file, err := parser.ParseFile(token.NewFileSet(), "plugin.go", src, 0)
	if err != nil {
		return false
	}
	if !importsDefaultGin(file) {
		return false
	}

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv == nil || fn.Name == nil || fn.Name.Name != "Register" {
			continue
		}
		if fn.Type == nil || countParams(fn.Type.Params) != 1 {
			continue
		}
		if isGinEnginePointer(fn.Type.Params.List[0].Type) {
			return true
		}
	}
	return false
}

func importsDefaultGin(file *ast.File) bool {
	for _, spec := range file.Imports {
		importPath, err := strconv.Unquote(spec.Path.Value)
		if err != nil || importPath != ginImportPath {
			continue
		}
		if spec.Name == nil {
			return true
		}
		return spec.Name.Name == "gin"
	}
	return false
}

func countParams(fields *ast.FieldList) int {
	if fields == nil {
		return 0
	}

	total := 0
	for _, field := range fields.List {
		if len(field.Names) == 0 {
			total++
			continue
		}
		total += len(field.Names)
	}
	return total
}

func FileImportsForbiddenModule(src []byte) bool {
	file, err := parser.ParseFile(token.NewFileSet(), "plugin.go", src, parser.ImportsOnly)
	if err != nil {
		return true
	}
	for _, spec := range file.Imports {
		importPath, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			return true
		}
		if isForbiddenPluginImport(importPath) {
			return true
		}
	}
	return false
}

func isForbiddenPluginImport(importPath string) bool {
	rest, ok := strings.CutPrefix(importPath, sparkopsModulePrefix+"modules/")
	if !ok {
		return false
	}
	_, kind, cut := strings.Cut(rest, "/")
	if !cut {
		return false
	}
	return strings.HasPrefix(kind, "domain") || strings.HasPrefix(kind, "infrastructure")
}

func isGinEnginePointer(expr ast.Expr) bool {
	ptr, ok := expr.(*ast.StarExpr)
	if !ok {
		return false
	}
	sel, ok := ptr.X.(*ast.SelectorExpr)
	if !ok || sel.Sel == nil || sel.Sel.Name != "Engine" {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && pkg.Name == "gin"
}
