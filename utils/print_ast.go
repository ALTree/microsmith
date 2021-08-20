package main

import (
	"go/ast"
	"go/parser"
	"go/token"
)

var src string = `
package p

func f[G1 I1, G2 I2]() {

}

func main() {
  f[int32, float32]()
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
