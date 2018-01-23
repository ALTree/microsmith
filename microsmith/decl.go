package microsmith

import (
	"fmt"
	"go/ast"
	"math/rand"
)

type DeclBuilder struct {
	rs *rand.Rand // randomness source
	sb *StmtBuilder

	// list of functions declared by this DeclBuilder
	// TODO: can this be removed(?)
	// (I don't think so, it's needed to avoid dups in func names)
	funNames map[string]struct{}
}

func NewDeclBuilder(seed int64) *DeclBuilder {
	db := new(DeclBuilder)
	db.rs = rand.New(rand.NewSource(seed))
	db.sb = NewStmtBuilder(db.rs)
	db.funNames = make(map[string]struct{})
	return db
}

func (db *DeclBuilder) FuncDecl() *ast.FuncDecl {
	fc := new(ast.FuncDecl)

	fc.Name = db.FuncIdent()

	fc.Type = &ast.FuncType{0, new(ast.FieldList), nil}
	fc.Body = db.sb.BlockStmt(0, 0)

	return fc
}

func (db *DeclBuilder) FuncIdent() *ast.Ident {

	id := new(ast.Ident)
	fns := db.funNames
	var name string
	name = fmt.Sprintf("fun%v", db.rs.Intn(100))
	for _, ok := fns[name]; ok; _, ok = fns[name] {
		name = fmt.Sprintf("fun%v", db.rs.Intn(100))
	}

	id.Obj = &ast.Object{Kind: ast.Fun, Name: name}
	fns[name] = struct{}{}
	db.funNames = fns

	id.Name = name
	return id
}

// returns *ast.File containing a package 'pName' and its source code,
// containing fCount functions.
func (db *DeclBuilder) File(pName string, fCount int) *ast.File {
	af := new(ast.File)

	af.Name = &ast.Ident{0, pName, nil}
	af.Decls = []ast.Decl{}
	for i := 0; i < fCount; i++ {
		af.Decls = append(af.Decls, db.FuncDecl())
	}

	// add empty main func
	if pName == "main" {
		mainF := &ast.FuncDecl{
			Name: &ast.Ident{Name: "main"},
			Type: &ast.FuncType{Params: &ast.FieldList{}},
			Body: &ast.BlockStmt{},
		}
		af.Decls = append(af.Decls, mainF)
	}

	return af
}
