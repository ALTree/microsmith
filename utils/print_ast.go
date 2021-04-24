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
  j := 1 + i
  f := func(i int) int {
    return i+1
  }
  defer f(7)
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
