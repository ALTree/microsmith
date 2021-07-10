package main

import (
	"go/ast"
	"go/parser"
	"go/token"
)

var src string = `
package main

func main() {
  var x [4]int
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
