package main

import (
	"go/ast"
	"go/parser"
	"go/token"
)

var src string = `
package main

import "fmt"

var i int = 33 + 1

func main() {
  j := 1 + i
}
`

func main() {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", src, 0)
	if err != nil {
		panic(err)
	}

	ast.Print(fset, f)
}
