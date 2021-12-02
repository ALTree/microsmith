package microsmith

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"math/rand"
	"strings"
)

type PackageBuilder struct {
	pb        *ProgramBuilder
	pkg       string
	ctx       *Context
	rs        *rand.Rand
	sb        *StmtBuilder
	eb        *ExprBuilder
	baseTypes []Type
	typedepth int
	funcs     []*ast.FuncDecl // top level funcs declared in the package
}

func NewPackageBuilder(conf ProgramConf, pkg string, progb *ProgramBuilder) *PackageBuilder {
	pb := PackageBuilder{
		pkg: pkg,
		ctx: NewContext(conf),
		rs:  rand.New(rand.NewSource(rand.Int63())),
		pb:  progb,
	}

	// Initialize Context.Scope with the predeclared functions:
	scope := make(Scope, 0, 64)
	for _, f := range PredeclaredFuncs {
		scope = append(scope, Variable{f, &ast.Ident{Name: f.Name()}})
	}
	pb.ctx.scope = &scope

	// Create the Stmt and Expr builders
	pb.sb = NewStmtBuilder(&pb)
	pb.eb = NewExprBuilder(&pb)

	// Add predeclared base types
	pb.baseTypes = []Type{
		BasicType{"int"},
		BasicType{"bool"},
		BasicType{"byte"},
		BasicType{"int8"},
		BasicType{"int16"},
		BasicType{"int32"},
		BasicType{"int64"},
		BasicType{"uint"},
		BasicType{"uintptr"},
		BasicType{"float32"},
		BasicType{"float64"},
		BasicType{"complex128"},
		BasicType{"rune"},
		BasicType{"string"},
	}
	if conf.TypeParams {
		pb.baseTypes = append(pb.baseTypes, BasicType{"any"})
	}

	return &pb
}

func (pb *PackageBuilder) FuncDecl() *ast.FuncDecl {

	fd := &ast.FuncDecl{
		Name: pb.FuncIdent(len(pb.funcs)),
		Type: &ast.FuncType{
			Func:    0,
			Params:  new(ast.FieldList),
			Results: nil,
		},
	}

	defer func() {
		// only available inside the function body
		pb.ctx.typeparams = nil
	}()

	// if not using typeparams, generate a body and return
	if !pb.Conf().TypeParams || pb.pkg != "main" {
		pb.sb.currfunc = fd
		fd.Body = pb.sb.BlockStmt()
		return fd
	}

	// If typeparams requested, use a few of the available one in the
	// function signature, and add them to scope.
	tp, tps := make(Scope, 0, 8), []*ast.Field{}
	for i := 0; i < 1+rand.Intn(8); i++ {
		ident := &ast.Ident{Name: fmt.Sprintf("G%v", i)}
		typ := pb.ctx.constraints[pb.rs.Intn(len(pb.ctx.constraints))]
		tps = append(
			tps,
			&ast.Field{Names: []*ast.Ident{ident}, Type: typ.N},
		)
		tp.AddVariable(ident, typ) // TODO(alb)
	}
	pb.ctx.typeparams = &tp

	fd.Type.TypeParams = &ast.FieldList{List: tps}
	pb.sb.currfunc = fd // this needs to be before the BlockStmt()
	fd.Body = pb.sb.BlockStmt()
	return fd
}

func (pb *PackageBuilder) FuncIdent(i int) *ast.Ident {
	id := new(ast.Ident)
	id.Obj = &ast.Object{
		Kind: ast.Fun,
		Name: fmt.Sprintf("F%v", i),
	}
	id.Name = id.Obj.Name

	return id
}

func (pb *PackageBuilder) Conf() ProgramConf {
	return pb.ctx.programConf
}

func (pb *PackageBuilder) Scope() *Scope {
	return pb.ctx.scope
}

func (pb *PackageBuilder) File() *ast.File {
	af := new(ast.File)
	af.Name = &ast.Ident{0, pb.pkg, nil}
	af.Decls = []ast.Decl{}

	if pb.pkg == "main" && pb.Conf().MultiPkg {
		af.Decls = append(af.Decls, MakeImport(fmt.Sprintf(`"%v_a"`, pb.pb.id)))
	}

	af.Decls = append(af.Decls, MakeImport(`"math"`))
	af.Decls = append(af.Decls, MakeImport(`"unsafe"`))

	// eg:
	//   var _ = math.Sqrt
	// (to avoid "unused package" errors)
	af.Decls = append(af.Decls, MakeUsePakage(`"math"`))
	af.Decls = append(af.Decls, MakeUsePakage(`"unsafe"`))

	tp := pb.Conf().TypeParams
	if tp {
		for i := 0; i < 1+rand.Intn(6); i++ {
			c, tp := pb.MakeRandConstraint(fmt.Sprintf("I%v", i))
			af.Decls = append(af.Decls, c)
			pb.ctx.constraints = append(pb.ctx.constraints, tp)
		}
	}

	// In the global scope:
	//   var i int
	// So we always have an int available
	af.Decls = append(af.Decls, MakeInt())
	pb.Scope().AddVariable(&ast.Ident{Name: "i"}, BasicType{"int"})

	// Now half a dozen top-level variables
	for i := 1; i <= 6; i++ {
		t := pb.baseTypes[rand.Intn(len(pb.baseTypes))]
		if pb.rs.Intn(3) == 0 {
			t = PointerOf(t)
		}
		if pb.rs.Intn(5) == 0 {
			t = ArrayOf(t)
		}
		af.Decls = append(af.Decls, pb.MakeVar(t, i))
		pb.Scope().AddVariable(&ast.Ident{Name: fmt.Sprintf("V%v", i)}, t)
	}

	// Declare top-level functions
	for i := 0; i < 4+pb.rs.Intn(5); i++ {
		fd := pb.FuncDecl()

		// append the function (decl and body) to the file
		af.Decls = append(af.Decls, fd)

		// save pointer to the decl in funcs, so we can list the
		// top level functions withoup having to loop on the whole
		// ast.File looking for func ast objects.
		pb.funcs = append(pb.funcs, fd)
	}

	// If we're not building a main package, we're done. Otherwise,
	// add a main func.
	if pb.pkg != "main" {
		return af
	}

	mainF := &ast.FuncDecl{
		Name: &ast.Ident{Name: "main"},
		Type: &ast.FuncType{Params: &ast.FieldList{}},
		Body: &ast.BlockStmt{},
	}

	// call all the functions declared in this package
	for _, f := range pb.funcs {
		var ce ast.CallExpr
		if tp {
			// for each typeparam attached to the function, find
			// its typelist, choose a subtype at random, and use
			// it in the call.
			var indices []ast.Expr
			for _, typ := range f.Type.TypeParams.List {
				types := FindByName(pb.ctx.constraints, typ.Type.(*ast.Ident).Name).Types
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

	// call the func in package a
	if pb.Conf().MultiPkg {
		var ce ast.CallExpr
		ce.Fun = &ast.SelectorExpr{
			X:   &ast.Ident{Name: "a"},
			Sel: pb.FuncIdent(0),
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
	case `"unsafe"`:
		// var _ = unsafe.Sizeof is not allowed, we need to call it.
		return &ast.GenDecl{
			Tok: token.VAR,
			Specs: []ast.Spec{
				&ast.ValueSpec{
					Names: []*ast.Ident{&ast.Ident{Name: "_"}},
					Values: []ast.Expr{&ast.CallExpr{
						Fun: &ast.SelectorExpr{
							X:   &ast.Ident{Name: "unsafe"},
							Sel: &ast.Ident{Name: "Sizeof"},
						},
						Args: []ast.Expr{&ast.Ident{Name: "0"}},
					}},
				},
			},
		}
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

func (pb *PackageBuilder) MakeRandConstraint(name string) (*ast.GenDecl, Constraint) {
	types := []Type{
		BasicType{"int"},
		BasicType{"byte"},
		BasicType{"int8"},
		BasicType{"int16"},
		BasicType{"int32"},
		BasicType{"int64"},
		BasicType{"uint"},
		BasicType{"uintptr"},
		BasicType{"float32"},
		BasicType{"float64"},
		BasicType{"string"},
	}

	pb.rs.Shuffle(len(types), func(i, j int) { types[i], types[j] = types[j], types[i] })
	types = types[:1+pb.rs.Intn(len(types)-1)]

	src := "package p\n"
	src += "type " + name + " interface{\n"
	for _, t := range types {
		if pb.rs.Intn(3) == 0 && t.Name() != "any" {
			src += "~"
		}
		src += t.Name() + "|"
	}
	src = strings.TrimRight(src, "|")
	src += "\n}"
	f, _ := parser.ParseFile(token.NewFileSet(), "", src, 0)
	decl := f.Decls[0].(*ast.GenDecl)

	return decl, Constraint{Types: types, N: &ast.Ident{Name: name}}
}

func (pb *PackageBuilder) MakeVar(t Type, i int) *ast.GenDecl {
	return &ast.GenDecl{
		Tok: token.VAR,
		Specs: []ast.Spec{
			&ast.ValueSpec{
				Names: []*ast.Ident{
					&ast.Ident{Name: fmt.Sprintf("V%v", i)},
				},
				Type: t.Ast(),
				Values: []ast.Expr{
					pb.eb.Expr(t),
				},
			},
		},
	}
}
