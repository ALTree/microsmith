package microsmith

import (
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"math/rand"
)

type ProgramConf struct {
	StmtConf       // defined in stmt.go
	SupportedTypes []Type
}

func RandConf() ProgramConf {
	pc := ProgramConf{
		StmtConf{MaxStmtDepth: 1 + rand.Intn(3)},
		nil,
	}

	// give each type a 0.75 chance to be enabled
	types := []Type{
		BasicType{"int"},
		BasicType{"float64"},
		BasicType{"complex128"},
		BasicType{"bool"},
		BasicType{"string"},
		BasicType{"rune"},
	}
	var enabledTypes []Type
	for _, t := range types {
		if rand.Float64() < 0.75 {
			enabledTypes = append(enabledTypes, t)
		}
	}
	pc.SupportedTypes = enabledTypes

	pc.Check(true) // fix conf without reporting errors
	return pc
}

type ConfError string

func (bce ConfError) Error() string {
	return fmt.Sprintf("Bad Conf: %s", bce)
}

func (pc *ProgramConf) Check(fix bool) error {
	// at least one type needs to be enabled
	if len(pc.SupportedTypes) == 0 {
		if fix {
			pc.SupportedTypes = []Type{
				BasicType{"int"},
				BasicType{"float64"},
				BasicType{"complex128"},
				BasicType{"bool"},
				BasicType{"string"},
			}
		} else {
			return errors.New("Bad Conf: len(EnabledTypes) is zero")
		}
	}

	return nil
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
func (db *DeclBuilder) File(pName string, fCount int) *ast.File {
	af := new(ast.File)
	af.Name = &ast.Ident{0, pName, nil}
	af.Decls = []ast.Decl{}

	af.Decls = append(af.Decls, MakeImport(`"math"`))

	// eg:
	//   var _ = math.Sqrt
	// (to avoid "unused package" errors)
	af.Decls = append(af.Decls, MakeUsePakage(`"math"`))

	// now, a few functions
	for i := 0; i < fCount; i++ {
		af.Decls = append(af.Decls, db.FuncDecl(i))
	}

	// finally, an empty main func
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
