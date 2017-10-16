package main

import (
	"bytes"
	"fmt"
	"go/printer"
	"go/token"
	"os"

	"github.com/ALTree/microsmith/microsmith"
)

func main() {

	fh, err := os.Create("test.go")
	if err != nil {
		fmt.Printf("could not create file: %s", err)
		return
	}

	gp := genFile("prova.go")

	fh.WriteString(gp)
	fmt.Println("Go program written to file")
	fmt.Println("--------\n", gp)
	fh.Close()
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
