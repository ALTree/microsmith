package microsmith

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"math/rand"
	"strings"
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
	FuncNum    int
	MultiPkg   bool
	TypeParams bool
}

func RandConf(rs *rand.Rand) ProgramConf {
	return ProgramConf{
		FuncNum: 8,
	}
}

func RandType() Type {
	return AllTypes[rand.Intn(len(AllTypes))]
}

type DeclBuilder struct {
	sb         *StmtBuilder
	typeparams TypeParams // Type Parameters available in the package
}

func NewDeclBuilder(rs *rand.Rand, conf ProgramConf) *DeclBuilder {
	return &DeclBuilder{sb: NewStmtBuilder(rs, conf)}
}

func (db *DeclBuilder) FuncDecl(i int, pkg string) *ast.FuncDecl {

	fd := &ast.FuncDecl{
		Name: db.FuncIdent(i),
		Type: &ast.FuncType{
			Func:    0,
			Params:  new(ast.FieldList),
			Results: nil,
		},
	}

	// if not using typeparams, generate a body and return
	if !db.Conf().TypeParams || pkg != "main" {
		db.sb.currfunc = fd
		fd.Body = db.sb.BlockStmt()
		return fd
	}

	// otherwise, add them
	tp, tps := db.typeparams, []*ast.Field{}
	for i := 0; i < 1+rand.Intn(3); i++ {
		tps = append(
			tps,
			&ast.Field{
				Names: []*ast.Ident{&ast.Ident{Name: fmt.Sprintf("G%v", i)}},
				Type:  tp[rand.Intn(len(tp))].N,
			},
		)
	}

	fd.Type.TypeParams = &ast.FieldList{List: tps}
	db.sb.currfunc = fd // this needs to be before the BlockStmt()
	fd.Body = db.sb.BlockStmt()
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
	if tp && pkg == "main" {
		for i := 0; i < 1+rand.Intn(6); i++ {
			c, tp := db.MakeRandConstraint(fmt.Sprintf("I%v", i))
			af.Decls = append(af.Decls, c)
			db.typeparams = append(db.typeparams, tp)
		}
	}
	db.sb.typeparams = db.typeparams

	// In the global scope:
	//   var i int
	// So we always have an int available
	af.Decls = append(af.Decls, MakeInt())
	db.sb.scope.AddVariable(&ast.Ident{Name: "i"}, BasicType{"int"})

	// Now half a dozen top-level variables
	for i := 1; i <= 6; i++ {
		t := RandType()
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
		af.Decls = append(af.Decls, db.FuncDecl(i, pkg))
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
	for _, decl := range af.Decls {
		if f, ok := decl.(*ast.FuncDecl); ok {
			var ce ast.CallExpr
			if tp {
				// for each typeparam attached to the function, find
				// its typelist, choose a subtype at random, and use
				// it in the call.
				var indices []ast.Expr
				for _, typ := range f.Type.TypeParams.List {
					types := db.typeparams.FindByName(typ.Type.(*ast.Ident).Name).Types
					indices = append(indices, types[rand.Intn(len(types))].Ast())
				}
				ce.Fun = &ast.IndexListExpr{X: f.Name, Indices: indices}
			} else {
				ce.Fun = f.Name
			}

			mainF.Body.List = append(
				mainF.Body.List,
				&ast.ExprStmt{&ce},
			)
		}
	}

	// call the func in package a
	if db.Conf().MultiPkg {
		var ce ast.CallExpr
		ce.Fun = &ast.SelectorExpr{
			X:   &ast.Ident{Name: "a"},
			Sel: db.FuncIdent(0),
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

func (db *DeclBuilder) MakeRandConstraint(name string) (*ast.GenDecl, TypeParam) {
	types := make([]Type, len(AllTypes))
	copy(types, AllTypes)
	db.sb.rs.Shuffle(len(types), func(i, j int) { types[i], types[j] = types[j], types[i] })

	types = types[:2+db.sb.rs.Intn(len(types)-2)]

	// runte overlaps with int32, not allowed in constraints. Must
	// remove rune if it's in the list.
	ri := -1
	for i := range types {
		if types[i].Name() == "rune" {
			ri = i
			break
		}
	}
	if ri != -1 {
		types = append(types[:ri], types[ri+1:]...)
	}

	src := "package p\n"
	src += "type " + name + " interface{\n"
	for _, t := range types {
		src += t.Name() + "|"
	}
	src = strings.TrimRight(src, "|")
	src += "\n}"
	f, _ := parser.ParseFile(token.NewFileSet(), "", src, 0)
	decl := f.Decls[0].(*ast.GenDecl)

	return decl, TypeParam{Types: types, N: &ast.Ident{Name: name}}
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
