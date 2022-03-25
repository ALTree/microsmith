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
	scope := Scope{pb: &pb, vars: make([]Variable, 0, 64)}
	for _, f := range PredeclaredFuncs {
		scope.vars = append(scope.vars, Variable{f, &ast.Ident{Name: f.N}})
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

	// if not using typeparams, generate a body and return
	if !pb.Conf().TypeParams {
		pb.sb.currfunc = fd
		fd.Body = pb.sb.BlockStmt()
		return fd
	}

	// If we're using type parameters, use a few of the available ones
	// in the function signature, and add them to body's scope.
	tp, tps := Scope{pb: pb, vars: make([]Variable, 0, 8)}, []*ast.Field{}
	tpDecl, tpVars := []ast.Stmt{}, []*ast.Ident{}
	for i := 0; i < 1+pb.rs.Intn(8); i++ {
		ident := &ast.Ident{Name: fmt.Sprintf("G%v", i)}
		typ := RandItem(pb.rs, pb.ctx.constraints)
		tps = append(
			tps,
			&ast.Field{Names: []*ast.Ident{ident}, Type: typ.N},
		)
		tp.AddVariable(ident, typ)

		// Collect DeclStmts of variables of the typeparameter's type,
		// like this:
		//
		//   var g0 G0
		//
		// Later we'll put these declarations at the beginning of the
		// function body, so we can use g0 when we need an expression
		// of type G0 but its type doesn't allow to create literals
		// (for example interface{ int | []int }).
		st, nv := pb.sb.DeclStmt(1, TypeParam{ident, typ})
		tpVars = append(tpVars, nv...)
		tpDecl = append(tpDecl, st)
	}
	pb.ctx.typeparams = &tp

	fd.Type.TypeParams = &ast.FieldList{List: tps}
	pb.sb.currfunc = fd // this needs to be before the BlockStmt()
	body := pb.sb.BlockStmt()

	// put the collected DeclStmts at the top of the body
	body.List = append(tpDecl, body.List...)

	// then append _ = ... to avoid unused variables errors.
	if len(tpVars) > 0 {
		body.List = append(body.List, pb.sb.UseVars(tpVars))
	}

	// finally, delete them from scope.
	for _, v := range tpVars {
		pb.sb.S().DeleteIdentByName(v)
	}

	fd.Body = body

	// Type parameters are only available inside the function body, so
	// clear them out when we're done generating the body.
	pb.ctx.typeparams = nil

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
		for _, p := range pb.pb.pkgs {
			if p.pkg == "main" {
				continue
			}
			af.Decls = append(af.Decls, MakeImport(fmt.Sprintf("%v_%s", pb.pb.id, p.pkg)))
		}
	}

	pkgs := []string{"math", "strings", "unsafe"}
	for _, p := range pkgs {
		af.Decls = append(af.Decls, MakeImport(p))
	}
	for _, p := range pkgs {
		af.Decls = append(af.Decls, MakeUsePakage(p))
	}

	if pb.Conf().TypeParams {
		for i := 0; i < 1+pb.rs.Intn(6); i++ {
			c, tp := pb.MakeRandConstraint(fmt.Sprintf("I%v", i))
			af.Decls = append(af.Decls, c)
			pb.ctx.constraints = append(pb.ctx.constraints, tp)
		}
	}

	// Outside any func:
	//   var i int
	// So we always have an int variable in scope.
	af.Decls = append(af.Decls, MakeInt())
	pb.Scope().AddVariable(&ast.Ident{Name: "i"}, BasicType{"int"})

	// Now half a dozen top-level variables
	for i := 1; i <= 6; i++ {
		t := RandItem(pb.rs, pb.baseTypes)
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

	// If we're not building the main package, we're done.
	if pb.pkg != "main" {
		return af
	}

	// build a main function
	mainF := &ast.FuncDecl{
		Name: &ast.Ident{Name: "main"},
		Type: &ast.FuncType{Params: &ast.FieldList{}},
		Body: &ast.BlockStmt{},
	}

	// call all the functions we declared
	for _, p := range pb.pb.pkgs {
		mainF.Body.List = append(mainF.Body.List, p.MakeFuncCalls()...)
	}

	af.Decls = append(af.Decls, mainF)
	return af
}

// Returns a slice of ast.ExprStms with calls to every top-level
// function of the receiver. Takes care of adding explicit type
// parameters, when the function has them.
func (p *PackageBuilder) MakeFuncCalls() []ast.Stmt {
	calls := make([]ast.Stmt, 0, len(p.funcs))
	for _, f := range p.funcs {
		var ce ast.CallExpr
		ce.Fun = f.Name

		// prepend <pkg> to F()
		if p.pkg != "main" {
			ce.Fun = &ast.SelectorExpr{
				X:   &ast.Ident{Name: p.pkg},
				Sel: f.Name,
			}
		}

		// instantiate type parameters at random
		if p.Conf().TypeParams {
			var indices []ast.Expr
			for _, typ := range f.Type.TypeParams.List {
				types := FindByName(p.ctx.constraints, typ.Type.(*ast.Ident).Name).Types
				indices = append(indices, RandItem(p.rs, types).Ast())
			}
			ce.Fun = &ast.IndexListExpr{X: ce.Fun, Indices: indices}
		}

		calls = append(calls, &ast.ExprStmt{&ce})
	}
	return calls
}

// Builds this:
//   import "<p>"
// p must be include a " char in the fist and last position.
func MakeImport(p string) *ast.GenDecl {
	return &ast.GenDecl{
		Tok: token.IMPORT,
		Specs: []ast.Spec{
			&ast.ImportSpec{
				Path: &ast.BasicLit{Kind: token.STRING, Value: "`" + p + "`"},
			},
		},
	}
}

func MakeUsePakage(p string) *ast.GenDecl {
	se := &ast.SelectorExpr{}
	switch p {
	case "unsafe":
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
		fs := map[string]string{
			"math":    "Sqrt",
			"strings": "Title",
		}
		se.X = &ast.Ident{Name: p}
		se.Sel = &ast.Ident{Name: fs[p]}
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
	var types []Type
	for len(types) < 1+pb.rs.Intn(8) {
		t := pb.RandType()
		if t.Contains(BasicType{"int32"}) { // conflicts with rune
			continue
		}
		same := false
		for _, t2 := range types {
			if t.Equal(t2) {
				same = true
				break
			}
		}
		if !same {
			types = append(types, t)
		}
	}

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

	f, err := parser.ParseFile(token.NewFileSet(), "", src, 0)
	if err != nil {
		panic("Parsing constraint failed:\n" + src + "\n\n" + err.Error())
	}
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
