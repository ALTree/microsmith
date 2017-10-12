package microsmith

import (
	"fmt"
	"go/ast"
	"math/rand"
)

type DeclBuilder struct {
	rs *rand.Rand // randomness source
	sb *StmtBuilder
}

func NewDeclBuilder(seed int64) *DeclBuilder {
	db := new(DeclBuilder)
	db.rs = rand.New(rand.NewSource(seed))
	db.sb = NewStmtBuilder(db.rs)
	return db
}

func (db *DeclBuilder) FuncDecl() *ast.FuncDecl {
	fc := new(ast.FuncDecl)

	fc.Name = db.FuncIdent()

	fc.Type = &ast.FuncType{0, new(ast.FieldList), nil}
	db.sb.currentFunc = fc.Name.Name
	fc.Body = db.sb.BlockStmt()

	fc.Body.List = append(fc.Body.List, db.sb.UseVars()...)

	return fc
}

var funNames map[string]function

func init() {
	funNames = make(map[string]function)
}

func (db *DeclBuilder) FuncIdent() *ast.Ident {

	id := new(ast.Ident)

	var name string
	name = fmt.Sprintf("fun%v", db.rs.Intn(1000))
	for _, ok := funNames[name]; ok; {
		name = fmt.Sprintf("fun%v", db.rs.Intn(1000))
	}
	id.Obj = &ast.Object{Kind: ast.Fun, Name: name}
	fn := function{name: id}
	fn.vars = make(map[string]*ast.Ident)
	funNames[name] = fn

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
