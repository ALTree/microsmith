package microsmith

import (
	"fmt"
	"go/ast"
	"go/token"
	"math/rand"
)

var AllTypes = []Type{
	BasicType{"int"}, // always enabled, leave in first position
	BasicType{"bool"},
	BasicType{"byte"},
	BasicType{"int8"},
	BasicType{"int16"},
	BasicType{"int32"},
	BasicType{"int64"},
	BasicType{"uint"},
	BasicType{"float32"},
	BasicType{"float64"},
	BasicType{"complex128"},
	BasicType{"rune"},
	BasicType{"string"},
}

type ProgramConf struct {
	StmtConf       // defined in stmt.go
	SupportedTypes []Type
}

func RandConf(rs *rand.Rand) ProgramConf {
	var pc ProgramConf
	pc.StmtConf = StmtConf{MaxStmtDepth: 1 + rand.Intn(3)}
	pc.SupportedTypes = []Type{AllTypes[0]}
	for _, t := range AllTypes[1:] {
		if rs.Float64() < 0.70 { // each type has a 0.70 chance to be enabled
			pc.SupportedTypes = append(pc.SupportedTypes, t)
		}
	}
	return pc
}

type DeclBuilder struct {
	sb *StmtBuilder
}

func NewDeclBuilder(rs *rand.Rand, conf ProgramConf) *DeclBuilder {
	return &DeclBuilder{sb: NewStmtBuilder(rs, conf)}
}

func (db *DeclBuilder) FuncDecl(i int) *ast.FuncDecl {
	return &ast.FuncDecl{
		Name: db.FuncIdent(i),
		Type: &ast.FuncType{0, new(ast.FieldList), nil},
		Body: db.sb.BlockStmt(),
	}
}

func (db *DeclBuilder) FuncIdent(i int) *ast.Ident {
	id := new(ast.Ident)
	id.Obj = &ast.Object{
		Kind: ast.Fun,
		Name: fmt.Sprintf("f%v", i),
	}
	id.Name = id.Obj.Name

	return id
}

// returns *ast.File containing a package 'pName' and its source code,
// containing fCount functions.
func (db *DeclBuilder) File(n int) *ast.File {
	af := new(ast.File)
	af.Name = &ast.Ident{0, "main", nil}
	af.Decls = []ast.Decl{}

	af.Decls = append(af.Decls, MakeImport(`"math"`))

	// eg:
	//   var _ = math.Sqrt
	// (to avoid "unused package" errors)
	af.Decls = append(af.Decls, MakeUsePakage(`"math"`))

	// In the global scope:
	//   var i int
	// So we always have an int available
	af.Decls = append(af.Decls, db.MakeInt())
	db.sb.scope.AddVariable(&ast.Ident{Name: "i"}, BasicType{"int"})

	// Now half a dozen top-level variables
	for i := 1; i <= 6; i++ {
		t := RandType(db.sb.conf.SupportedTypes)
		if db.sb.rs.Intn(3) == 0 {
			t = PointerOf(t)
		}
		if db.sb.rs.Intn(5) == 0 {
			t = ArrayOf(t)
		}
		af.Decls = append(af.Decls, db.MakeVar(t, i))
		db.sb.scope.AddVariable(&ast.Ident{Name: fmt.Sprintf("V%v", i)}, t)
	}

	// A few functions
	for i := 0; i < n; i++ {
		af.Decls = append(af.Decls, db.FuncDecl(i))
	}

	// finally, the main function
	mainF := &ast.FuncDecl{
		Name: &ast.Ident{Name: "main"},
		Type: &ast.FuncType{Params: &ast.FieldList{}},
		Body: &ast.BlockStmt{},
	}
	for i := 0; i < n; i++ {
		mainF.Body.List = append(
			mainF.Body.List,
			&ast.ExprStmt{&ast.CallExpr{Fun: db.FuncIdent(i)}})
	}
	af.Decls = append(af.Decls, mainF)

	return af
}

// Builds this:
//   import "<p>"
// p must be include a " char in the 1st and last position.
func MakeImport(p string) *ast.GenDecl {
	return &ast.GenDecl{
		Tok: token.IMPORT,
		Specs: []ast.Spec{
			&ast.ImportSpec{
				Path: &ast.BasicLit{Kind: token.STRING, Value: p},
			},
		},
	}
}

func MakeUsePakage(p string) *ast.GenDecl {
	se := &ast.SelectorExpr{}
	switch p {
	case `"math"`:
		se.X = &ast.Ident{Name: "math"}
		se.Sel = &ast.Ident{Name: "Sqrt"}
	default:
		panic("MakeUsePackage: bad package " + p)
	}

	return &ast.GenDecl{
		Tok: token.VAR,
		Specs: []ast.Spec{
			&ast.ValueSpec{
				Names:  []*ast.Ident{&ast.Ident{Name: "_"}},
				Values: []ast.Expr{se},
			},
		},
	}
}

func (db *DeclBuilder) MakeInt() *ast.GenDecl {
	return &ast.GenDecl{
		Tok: token.VAR,
		Specs: []ast.Spec{
			&ast.ValueSpec{
				Names: []*ast.Ident{
					&ast.Ident{Name: "i"},
				},
				Type: &ast.Ident{Name: "int"},
			},
		},
	}
}

func (db *DeclBuilder) MakeVar(t Type, i int) *ast.GenDecl {
	return &ast.GenDecl{
		Tok: token.VAR,
		Specs: []ast.Spec{
			&ast.ValueSpec{
				Names: []*ast.Ident{
					&ast.Ident{Name: fmt.Sprintf("V%v", i)},
				},
				Type: TypeIdent(t.Name()),
				Values: []ast.Expr{
					db.sb.eb.Expr(t),
				},
			},
		},
	}
}
