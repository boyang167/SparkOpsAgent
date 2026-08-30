package install

import (
	"go/ast"
	"go/parser"
	"go/token"
)

func FileDeclaresRegisterEngine(src []byte) bool {
	file, err := parser.ParseFile(token.NewFileSet(), "plugin.go", src, 0)
	if err != nil {
		return false
	}

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv == nil || fn.Name == nil || fn.Name.Name != "Register" {
			continue
		}
		if fn.Type == nil || fn.Type.Params == nil || len(fn.Type.Params.List) != 1 {
			continue
		}
		if isGinEnginePointer(fn.Type.Params.List[0].Type) {
			return true
		}
	}
	return false
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
