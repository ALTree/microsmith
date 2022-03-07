package main

import (
	"go/ast"
	"go/parser"
	"go/token"
)

var src string = `
package p

func f() {
  a = make([]int, 0)
  b = make(map[int]string, 0)
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
