// Check domain packages do not import infrastructure frameworks.
package main

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

var banned = []string{
	"gorm.io/gorm",
	"github.com/gin-gonic/gin",
	"net/http",
}

func main() {
	root := filepath.Join("internal", "modules")
	var violations []string

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		if !strings.Contains(path, string(filepath.Separator)+"domain"+string(filepath.Separator)) {
			return nil
		}

		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			violations = append(violations, fmt.Sprintf("parse %s: %v", path, err))
			return nil
		}
		for _, imp := range f.Imports {
			val := strings.Trim(imp.Path.Value, `"`)
			for _, b := range banned {
				if val == b || strings.HasPrefix(val, b+"/") {
					violations = append(violations, fmt.Sprintf("%s imports %s", path, val))
				}
			}
		}
		return nil
	})
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if len(violations) > 0 {
		fmt.Println("domain dependency violations:")
		for _, v := range violations {
			fmt.Println(" -", v)
		}
		os.Exit(1)
	}
	fmt.Println("domain dependency check passed")
}
