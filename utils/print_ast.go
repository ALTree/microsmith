package main

import (
	"go/ast"
	"go/parser"
	"go/token"
)

var src string = `
package main

import "fmt"

func main() {
  var m map[int]string
  m = map[int]string{1: "hi"}
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
