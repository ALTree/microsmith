package microsmith

import (
	"fmt"
	"go/ast"
	"math/rand"
)

var SupportedTypes = []Type{
	TypeInt,
	TypeBool,
	TypeString,
}

type ProgramConf struct {
	Stmt StmtConf // defined in stmt.go
	Expr ExprConf // defined in expr.go
}

var DefaultConf = ProgramConf{
	StmtConf{
		MaxStmtDepth: 2,
		StmtKindChance: []float64{
			2, 1, 1, 1, 1,
		},
		MaxBlockVars:  len(SupportedTypes),
		MaxBlockStmts: 8,
		UseArrays:     true,
	},
	ExprConf{
		UnaryChance:      0.1,
		LiteralChance:    0.2,
		ComparisonChance: 0.1,
		IndexChance:      0.1,
	},
}

type DeclBuilder struct {
	rs *rand.Rand // randomness source
	sb *StmtBuilder

	// list of function names declared by this DeclBuilder
	funNames []string
}

func NewDeclBuilder(seed int64, conf ProgramConf) *DeclBuilder {
	db := new(DeclBuilder)
	db.rs = rand.New(rand.NewSource(seed))
	db.sb = NewStmtBuilder(db.rs, conf)
	db.funNames = []string{}
	return db
}

func (db *DeclBuilder) FuncDecl() *ast.FuncDecl {
	fc := new(ast.FuncDecl)

	fc.Name = db.FuncIdent()

	fc.Type = &ast.FuncType{0, new(ast.FieldList), nil}

	// Call BlockStmt with 4 as first parameter so that we're sure
	// that at the beginning of the function 4 variables of each type
	// will be in scope.
	fc.Body = db.sb.BlockStmt(4*len(SupportedTypes), 0)

	return fc
}

func (db *DeclBuilder) FuncIdent() *ast.Ident {
	id := new(ast.Ident)

	name := fmt.Sprintf("fun%v", len(db.funNames))
	db.funNames = append(db.funNames, name)

	id.Obj = &ast.Object{
		Kind: ast.Fun,
		Name: name,
	}
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
