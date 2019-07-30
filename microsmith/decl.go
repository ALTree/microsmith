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
	ExprConf       // defined in expr.go
	SupportedTypes []Type
}

var DefaultConf = ProgramConf{
	StmtConf{
		MaxStmtDepth: 2,
		StmtKindChance: []float64{
			1, 1, 1, 1, 1, 1, 1,
		},
		MaxBlockVars:  8,
		MaxBlockStmts: 6,
		UseStructs:    true,
		UseChans:      true,
	},
	ExprConf{
		ExprKindChance: []float64{
			1, 1, 1,
		},
		LiteralChance:    0.2,
		ComparisonChance: 0.2,
		IndexChance:      0.2,
	},
	[]Type{
		BasicType{"int"},
		BasicType{"float64"},
		BasicType{"complex128"},
		BasicType{"bool"},
		BasicType{"string"},
	},
}

func RandConf() ProgramConf {
	pc := ProgramConf{
		StmtConf{
			MaxStmtDepth: 1 + rand.Intn(2),
			StmtKindChance: []float64{
				float64(rand.Intn(5)), // assign stms
				float64(rand.Intn(3)), // block stms
				float64(rand.Intn(5)), // for stms
				float64(rand.Intn(5)), // if stms
				float64(rand.Intn(5)), // switch stms
				float64(rand.Intn(1)), // inc and dec stms
				float64(rand.Intn(1)), // send stmts
			},

			// since the Stmt builder already calls rand(1,Max) to
			// decide how many variables and statements actually use,
			// there's no need to randomly vary the upper limits too.
			MaxBlockVars:  10,
			MaxBlockStmts: 6,

			UseStructs: rand.Int63()%2 == 0,
		},
		ExprConf{
			ExprKindChance: []float64{
				float64(rand.Intn(3)), // unary expr
				float64(rand.Intn(6)), // binary expr
				float64(rand.Intn(3)), // fun call
			},
			LiteralChance:    float64(rand.Intn(7)) * 0.125,
			ComparisonChance: float64(rand.Intn(7)) * 0.125,
			IndexChance:      float64(rand.Intn(5)) * 0.125,
		},
		nil,
	}

	// give each type a 0.75 chance to be enabled
	types := []Type{
		BasicType{"int"},
		BasicType{"float64"},
		BasicType{"complex128"},
		BasicType{"bool"},
		BasicType{"string"},
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
	// IndexChance cannot be zero, since we always generate arrays
	if pc.IndexChance == 0 {
		if fix {
			pc.IndexChance = 0.2
		} else {
			return errors.New("Bad Conf: Expr.IndexChance = 0")
		}
	}

	// LiteralChance cannot be 0 if we generate arrays, because when
	// we need a literal or a variable of type int to stop descending
	// into an infinite sequence of nested []. If we don't have an int
	// variable in scope and we don't allow literals, we'll get stuck
	// in a infinite mutual recursion between VarOrLit and IndexExpr.
	if pc.LiteralChance == 0 {
		if fix {
			pc.LiteralChance = 0.2
		} else {
			return errors.New("Bad Conf: Expr.LiteralChance = 0")
		}
	}

	if pc.IndexChance == 1 && pc.LiteralChance == 0 {
		if fix {
			pc.LiteralChance = 0.2
		} else {
			return errors.New("Bad Conf: Expr.IndexChance = 1, Expr.LiteralChance = 0")
		}
	}

	// StmtKindChance cannot be all zeros
	sum := 0.0
	for _, v := range pc.StmtKindChance {
		sum += v
	}
	if sum == 0 {
		if fix {
			for i := range pc.StmtKindChance {
				pc.StmtKindChance[i] += 1.0
			}
		} else {
			return errors.New("Bad Conf: StmtKindChance is all zeros")
		}
	}

	// ExprKindChance cannot be all zeros
	sum = 0.0
	for _, v := range pc.ExprKindChance {
		sum += v
	}
	if sum == 0 {
		if fix {
			for i := range pc.ExprKindChance {
				pc.ExprKindChance[i] += 1.0
			}
		} else {
			return errors.New("Bad Conf: ExprKindChance is all zeros")
		}
	}

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
	rs *rand.Rand
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
	fc.Body = db.sb.BlockStmt()
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

	af.Decls = append(af.Decls, MakeImport(`"math"`))
	af.Decls = append(af.Decls, MakeImport(`"math/rand"`))

	// eg:
	//   var _ = math.Sqrt
	// (to avoid "unused package" errors)
	af.Decls = append(af.Decls, MakeUsePakage(`"math"`))
	af.Decls = append(af.Decls, MakeUsePakage(`"math/rand"`))

	// now, a few functions
	for i := 0; i < fCount; i++ {
		af.Decls = append(af.Decls, db.FuncDecl())
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
	case `"math/rand"`:
		se.X = &ast.Ident{Name: "rand"}
		se.Sel = &ast.Ident{Name: "Int"}
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
