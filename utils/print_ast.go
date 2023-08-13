package main

import (
	"go/ast"
	"go/parser"
	"go/token"
)

var src string = `
package p

func f() {
  var i interface {
    M1(string) int
    M2(int) string
  }
  i.M1("hello")
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
