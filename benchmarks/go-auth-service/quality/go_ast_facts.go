package main

import (
	"encoding/json"
	"flag"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
)

type typeFact struct {
	Name  string `json:"name"`
	Alias bool   `json:"alias"`
}

type facts struct {
	Types     []typeFact `json:"types"`
	Functions []string   `json:"functions"`
	Methods   []string   `json:"methods"`
	Strings   []string   `json:"strings"`
	Selectors []string   `json:"selectors"`
}

func main() {
	project := flag.String("project", ".", "Go project root")
	flag.Parse()
	result := facts{}
	fset := token.NewFileSet()
	err := filepath.WalkDir(*project, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == ".execution-state" || entry.Name() == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" || filepath.Base(path) == "go_ast_facts.go" {
			return nil
		}
		file, parseErr := parser.ParseFile(fset, path, nil, 0)
		if parseErr != nil {
			return parseErr
		}
		for _, declaration := range file.Decls {
			switch value := declaration.(type) {
			case *ast.GenDecl:
				if value.Tok != token.TYPE {
					continue
				}
				for _, spec := range value.Specs {
					if typeSpec, ok := spec.(*ast.TypeSpec); ok {
						result.Types = append(result.Types, typeFact{Name: typeSpec.Name.Name, Alias: typeSpec.Assign.IsValid()})
					}
				}
			case *ast.FuncDecl:
				if value.Recv == nil {
					result.Functions = append(result.Functions, value.Name.Name)
				} else {
					result.Methods = append(result.Methods, value.Name.Name)
				}
			}
		}
		ast.Inspect(file, func(node ast.Node) bool {
			switch value := node.(type) {
			case *ast.BasicLit:
				if value.Kind == token.STRING {
					if decoded, decodeErr := strconv.Unquote(value.Value); decodeErr == nil {
						result.Strings = append(result.Strings, decoded)
					}
				}
			case *ast.SelectorExpr:
				if identifier, ok := value.X.(*ast.Ident); ok {
					result.Selectors = append(result.Selectors, identifier.Name+"."+value.Sel.Name)
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		_ = json.NewEncoder(os.Stdout).Encode(map[string]string{"error": err.Error()})
		os.Exit(2)
	}
	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		os.Exit(2)
	}
}
