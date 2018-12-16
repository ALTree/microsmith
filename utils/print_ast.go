package main

import (
	"go/ast"
	"go/parser"
	"go/token"
)

func main() {
	// src is the input for which we want to print the AST.
	src := `
package main

func f() {
  var c chan float64
  make(chan int) <- 1

}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", src, 0)
	if err != nil {
		panic(err)
	}

	ast.Print(fset, f)
}
