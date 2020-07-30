package main

import (
	"go/ast"
	"go/parser"
	"go/token"
)

var src string = `
package main

func main() {
label:
  for i := 0; i < 10; i++ {
    if i == 7 {
      goto label
    }
  }
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
