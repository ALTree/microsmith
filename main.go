package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"

	"github.com/ALTree/microsmith/microsmith"
)

func main() {
	// src is the input for which we want to print the AST.
	src := `
package p

func sum() {
  b := a
}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", src, 0)
	if err != nil {
		panic(err)
	}
	_, _ = f, fset
	ast.Print(fset, f)
	fmt.Println("\n----------------\n")

	fmt.Print(genFile("prova.go"))

}

func genFile(path string) string {
	db := microsmith.NewDeclBuilder(32)
	var buf bytes.Buffer
	printer.Fprint(&buf, token.NewFileSet(), db.File("main", 1))
	return buf.String()
}

// type File struct {
// 	Doc        *CommentGroup   // associated documentation; or nil
// 	Package    token.Pos       // position of "package" keyword
// 	Name       *Ident          // package name
// 	Decls      []Decl          // top-level declarations; or nil
// 	Scope      *Scope          // package scope (this file only)
// 	Imports    []*ImportSpec   // imports in this file
// 	Unresolved []*Ident        // unresolved identifiers in this file
// 	Comments   []*CommentGroup // list of all comments in the source file
// }
