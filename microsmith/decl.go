package microsmith

import (
	"fmt"
	"go/ast"
	"go/parser"
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
	StmtConf
	FuncNum    int
	MultiPkg   bool
	TypeParams bool
}

func RandConf(rs *rand.Rand) ProgramConf {
	return ProgramConf{
		StmtConf: StmtConf{MaxStmtDepth: 1 + rand.Intn(3)},
		FuncNum:  8,
	}
}

func (pc *ProgramConf) RandType() Type {
	return AllTypes[rand.Intn(len(AllTypes))]
}

type DeclBuilder struct {
	sb *StmtBuilder
}

func NewDeclBuilder(rs *rand.Rand, conf ProgramConf) *DeclBuilder {
	return &DeclBuilder{sb: NewStmtBuilder(rs, conf)}
}

func (db *DeclBuilder) FuncDecl(i int, n int) *ast.FuncDecl {

	fd := &ast.FuncDecl{
		Name: db.FuncIdent(i),
		Type: &ast.FuncType{
			Func:    0,
			Params:  new(ast.FieldList),
			Results: nil,
		},
		Body: db.sb.BlockStmt(),
	}

	// if not using typeparams, we're done
	if !db.Conf().TypeParams {
		return fd
	}

	// otherwise, add a TypeParams field
	tps := []*ast.Field{}
	for i := 0; i < n; i++ {
		tps = append(
			tps,
			&ast.Field{
				Names: []*ast.Ident{&ast.Ident{Name: fmt.Sprintf("G%v", i)}},
				Type:  &ast.Ident{Name: fmt.Sprintf("I%v", i)},
			},
		)
	}

	fd.Type.TypeParams = &ast.FieldList{List: tps}
	return fd
}

func (db *DeclBuilder) FuncIdent(i int) *ast.Ident {
	id := new(ast.Ident)
	id.Obj = &ast.Object{
		Kind: ast.Fun,
		Name: fmt.Sprintf("F%v", i),
	}
	id.Name = id.Obj.Name

	return id
}

func (db *DeclBuilder) Conf() ProgramConf {
	return db.sb.conf
}

func (db *DeclBuilder) File(pkg string, id uint64) *ast.File {
	af := new(ast.File)
	af.Name = &ast.Ident{0, pkg, nil}
	af.Decls = []ast.Decl{}

	if pkg == "main" && db.Conf().MultiPkg {
		af.Decls = append(af.Decls, MakeImport(fmt.Sprintf(`"prog%v_a"`, id)))
	}

	af.Decls = append(af.Decls, MakeImport(`"math"`))

	// eg:
	//   var _ = math.Sqrt
	// (to avoid "unused package" errors)
	af.Decls = append(af.Decls, MakeUsePakage(`"math"`))

	tp := db.Conf().TypeParams
	if tp {
		af.Decls = append(af.Decls, MakeConstraint("I0", "int8|int16|int32|int64|int|uint"))
		af.Decls = append(af.Decls, MakeConstraint("I1", "float32|float64"))
		af.Decls = append(af.Decls, MakeConstraint("I2", "string"))
	}

	// In the global scope:
	//   var i int
	// So we always have an int available
	af.Decls = append(af.Decls, MakeInt())
	db.sb.scope.AddVariable(&ast.Ident{Name: "i"}, BasicType{"int"})

	// Now half a dozen top-level variables
	for i := 1; i <= 6; i++ {
		t := db.sb.RandType()
		if db.sb.rs.Intn(3) == 0 {
			t = PointerOf(t)
		}
		if db.sb.rs.Intn(5) == 0 {
			t = ArrayOf(t)
		}
		af.Decls = append(af.Decls, db.MakeVar(t, i))
		db.sb.scope.AddVariable(&ast.Ident{Name: fmt.Sprintf("V%v", i)}, t)
	}

	fcnt := db.Conf().FuncNum
	if pkg != "main" {
		fcnt = 1
	}

	// Declare fcnt top-level functions
	for i := 0; i < fcnt; i++ {
		af.Decls = append(af.Decls, db.FuncDecl(i, 3))
	}

	// If we're not building a main package, we're done. Otherwise,
	// add a main func.
	if pkg != "main" {
		return af
	}

	mainF := &ast.FuncDecl{
		Name: &ast.Ident{Name: "main"},
		Type: &ast.FuncType{Params: &ast.FieldList{}},
		Body: &ast.BlockStmt{},
	}

	// call all local functions
	for i := 0; i < fcnt; i++ {
		var ce ast.CallExpr
		if tp {
			ce.Fun = &ast.IndexListExpr{
				X:       db.FuncIdent(i),
				Indices: []ast.Expr{&ast.Ident{Name: "int16"}, &ast.Ident{Name: "float32"}, &ast.Ident{Name: "string"}},
			}
		} else {
			ce.Fun = db.FuncIdent(i)
		}

		mainF.Body.List = append(
			mainF.Body.List,
			&ast.ExprStmt{&ce},
		)
	}

	// call the func in package a
	if db.Conf().MultiPkg {
		var ce ast.CallExpr
		if tp {
			ce.Fun = &ast.IndexListExpr{
				X:       &ast.SelectorExpr{X: &ast.Ident{Name: "a"}, Sel: db.FuncIdent(0)},
				Indices: []ast.Expr{&ast.Ident{Name: "int"}, &ast.Ident{Name: "float32"}, &ast.Ident{Name: "string"}},
			}
		} else {
			ce.Fun = &ast.SelectorExpr{
				X:   &ast.Ident{Name: "a"},
				Sel: db.FuncIdent(0),
			}
		}

		mainF.Body.List = append(
			mainF.Body.List,
			&ast.ExprStmt{&ce},
		)
	}

	af.Decls = append(af.Decls, mainF)
	return af
}

// Builds this:
//   import "<p>"
// p must be include a " char in the fist and last position.
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

func MakeInt() *ast.GenDecl {
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

func MakeConstraint(name, types string) *ast.GenDecl {
	src := "package p\n"
	src += "type " + name + " interface{\n"
	src += types + "\n}"
	f, _ := parser.ParseFile(token.NewFileSet(), "", src, 0)
	return f.Decls[0].(*ast.GenDecl)
}

func (db *DeclBuilder) MakeVar(t Type, i int) *ast.GenDecl {
	return &ast.GenDecl{
		Tok: token.VAR,
		Specs: []ast.Spec{
			&ast.ValueSpec{
				Names: []*ast.Ident{
					&ast.Ident{Name: fmt.Sprintf("V%v", i)},
				},
				Type: t.Ast(),
				Values: []ast.Expr{
					db.sb.eb.Expr(t),
				},
			},
		},
	}
}
