package microsmith

import (
	"fmt"
	"go/ast"
	"go/token"
	"math/rand"
	"strconv"
)

type StmtBuilder struct {
	conf       ProgramConf
	currfunc   *ast.FuncDecl // signature of func we're in
	rs         *rand.Rand    // randomness source
	eb         *ExprBuilder
	depth      int // how deep the stmt hyerarchy is
	scope      *Scope
	typeparams []TypeParam // type parameters available to the program
	funcp      int         // counter for function param names
	inloop     bool        // are we inside a loop?
	labels     []string
	label      int // counter for labels names
}

// Returns true if the block statement currently being built is
// allowed to have statements nested inside it.
func (sb *StmtBuilder) CanNest() bool {
	return (sb.depth <= 3) && (sb.eb.rs.Float64() < 0.8)
}

var InitialScope Scope

func init() {
	InitialScope = make(Scope, 0, len(PredeclaredFuncs))
	for _, f := range PredeclaredFuncs {
		InitialScope = append(InitialScope, Variable{f, &ast.Ident{Name: f.Name()}})
	}
}

func NewStmtBuilder(rs *rand.Rand, conf ProgramConf) *StmtBuilder {
	sb := new(StmtBuilder)
	sb.rs, sb.conf = rs, conf
	scope := make(Scope, 0, 32)
	scope = append(scope, InitialScope...)
	sb.scope = &scope
	sb.eb = NewExprBuilder(rs, conf, sb.scope)
	return sb
}

func (sb *StmtBuilder) Stmt() ast.Stmt {

	// Assign statements don't deepen the hyerarchy.
	if !sb.CanNest() {
		return sb.AssignStmt()
	}

	switch sb.rs.Intn(10) {
	case 0:
		return sb.AssignStmt()
	case 1:
		return sb.BlockStmt()
	case 2:
		// If at least one array or string is in scope, generate a for
		// range loop with chance 0.5; otherwise generate a plain loop
		arr, ok := sb.scope.GetRandomRangeable(sb.rs)
		if ok && sb.rs.Intn(2) == 0 {
			if sb.rs.Intn(4) == 0 { // 1 in 4 loops have a label
				sb.label++
				label := fmt.Sprintf("lab%v", sb.label)
				sb.labels = append(sb.labels, label)
				fs := &ast.LabeledStmt{
					Label: &ast.Ident{Name: label},
					Stmt:  sb.RangeStmt(arr),
				}
				return fs
			} else {
				return sb.RangeStmt(arr)
			}
		} else {
			if sb.rs.Intn(4) == 0 { // 1 in 4 loops have a label
				sb.label++
				label := fmt.Sprintf("lab%v", sb.label)
				sb.labels = append(sb.labels, label)
				fs := &ast.LabeledStmt{
					Label: &ast.Ident{Name: label},
					Stmt:  sb.ForStmt(),
				}
				return fs
			} else {
				return sb.ForStmt()
			}
		}
	case 3:
		return sb.IfStmt()
	case 4:
		return sb.SwitchStmt()
	case 5:
		return sb.SendStmt()
	case 6:
		return sb.SelectStmt()
	case 7:
		if sb.inloop {
			return sb.BranchStmt()
		}
		return sb.AssignStmt()
	case 8:
		return sb.DeferStmt()
	case 9:
		return sb.ExprStmt()
	default:
		panic("bad Stmt index")
	}
}

// gets a random variable currently in scope (that we can assign to),
// and builds an AssignStmt with a random Expr of its type on the RHS
func (sb *StmtBuilder) AssignStmt() *ast.AssignStmt {
	v := sb.scope.RandomVar(true)

	switch t := v.Type.(type) {

	case StructType:
		// For structs, 50/50 between assigning to the variable and
		// setting one of its fields.
		if sb.rs.Intn(2) == 0 || len(t.Ftypes) == 0 {
			// v = struct{<expr>, <expr>, ...}
			return &ast.AssignStmt{
				Lhs: []ast.Expr{v.Name},
				Tok: token.ASSIGN,
				Rhs: []ast.Expr{sb.eb.CompositeLit(t)},
			}
		} else {
			// v.field = <expr>
			fieldType := t.Ftypes[sb.rs.Intn(len(t.Fnames))]
			return &ast.AssignStmt{
				Lhs: []ast.Expr{sb.eb.StructFieldExpr(v.Name, t, fieldType)},
				Tok: token.ASSIGN,
				Rhs: []ast.Expr{sb.eb.Expr(fieldType)},
			}
		}

	case ArrayType:
		// For arrays, 50/50 between
		//   A[<expr>] = <expr>
		//   A = { <expr>, <expr>, ... }
		if sb.rs.Intn(2) == 0 {
			return &ast.AssignStmt{
				Lhs: []ast.Expr{sb.eb.IndexExpr(v.Name)},
				Tok: token.ASSIGN,
				Rhs: []ast.Expr{sb.eb.Expr(t.Base())},
			}
		} else {
			return &ast.AssignStmt{
				Lhs: []ast.Expr{v.Name},
				Tok: token.ASSIGN,
				Rhs: []ast.Expr{sb.eb.Expr(v.Type)},
			}
		}

	case ChanType:
		panic("AssignStmt: requested addressable, got chan")

	case MapType:
		// For maps, 50/50 between
		//   M[<expr>] = <expr>
		//   M = { <expr>: <expr> }
		if sb.rs.Intn(2) == 0 {
			return &ast.AssignStmt{
				Lhs: []ast.Expr{sb.eb.MapIndexExpr(v.Name, v.Type.(MapType).KeyT)},
				Tok: token.ASSIGN,
				Rhs: []ast.Expr{sb.eb.Expr(v.Type.(MapType).ValueT)},
			}
		} else {
			return &ast.AssignStmt{
				Lhs: []ast.Expr{v.Name},
				Tok: token.ASSIGN,
				Rhs: []ast.Expr{sb.eb.Expr(v.Type)},
			}
		}

	case FuncType:
		panic("AssignStmt: not for functions")

	default:
		return &ast.AssignStmt{
			Lhs: []ast.Expr{v.Name},
			Tok: token.ASSIGN,
			Rhs: []ast.Expr{sb.eb.Expr(v.Type)},
		}
	}
}

// returns a continue/break statement
func (sb *StmtBuilder) BranchStmt() *ast.BranchStmt {
	var bs ast.BranchStmt

	switch sb.rs.Intn(3) {
	case 0:
		bs.Tok = token.GOTO
	case 1:
		bs.Tok = token.CONTINUE
	case 2:
		bs.Tok = token.BREAK
	}

	// break/continue/goto to a label with chance 0.25
	if len(sb.labels) > 0 && sb.rs.Intn(4) == 0 {
		li := sb.rs.Intn(len(sb.labels))
		bs.Label = &ast.Ident{Name: sb.labels[li]}
		sb.labels = append(sb.labels[:li], sb.labels[li+1:]...)
	} else {
		// If we didn't add a label, GOTO is not allowed.
		if sb.rs.Intn(2) == 0 {
			bs.Tok = token.BREAK
		} else {
			bs.Tok = token.CONTINUE
		}
	}

	return &bs

}

// BlockStmt returns a new Block Statement. The returned Stmt is
// always a valid block. It up to BlockStmt's caller to make sure
// BlockStmt is only called when we have not yet reached max depth.
func (sb *StmtBuilder) BlockStmt() *ast.BlockStmt {

	sb.depth++
	defer func() { sb.depth-- }()

	bs := new(ast.BlockStmt)
	stmts := []ast.Stmt{}

	// A new block means opening a new scope. Declare a few new vars
	// of random types.
	var newVars []*ast.Ident
	for _, t := range sb.RandomTypes(3 + sb.rs.Intn(6)) {
		newDecl, nv := sb.DeclStmt(1+sb.rs.Intn(3), t)
		stmts = append(stmts, newDecl)
		newVars = append(newVars, nv...)
	}

	// And a few vars of the type parameters available in the
	// function.
	if sb.currfunc.Type.TypeParams != nil {
		for i := 0; i < 1+sb.rs.Intn(3); i++ {
			randTypeParam := sb.typeparams[0]
			ds, idents := sb.DeclStmt(1+sb.rs.Intn(2), randTypeParam)
			stmts = append(stmts, ds)
			newVars = append(newVars, idents...)
		}
	}

	var nStmts int
	if !sb.CanNest() {
		// If we stop nesting statements, guarantee 8 assignmens, to
		// so we don't generate almost-empty blocks.
		nStmts = 8
	} else {
		nStmts = 6 + sb.rs.Intn(5)
	}

	// Fill the block's body.
	for i := 0; i < nStmts; i++ {
		stmts = append(stmts, sb.Stmt())
	}

	if len(newVars) > 0 {
		stmts = append(stmts, sb.UseVars(newVars))
	}

	for _, v := range newVars {
		sb.scope.DeleteIdentByName(v)
	}

	bs.List = stmts
	return bs
}

func (sb *StmtBuilder) RandomTypes(n int) []Type {
	if n < 1 {
		panic("n < 1")
	}

	types := make([]Type, 0, n)
	for i := 0; i < n; i++ {
		types = append(types, sb.RandomType(false))
	}

	return types
}

func (sb *StmtBuilder) RandomType(comp bool) Type {
	switch sb.rs.Intn(14) {
	case 0, 1:
		if comp {
			return RandType()
		} else {
			return ArrayOf(sb.RandomType(true))
		}
	case 2:
		return ChanOf(sb.RandomType(true))
	case 3, 4:
		return MapOf(
			RandType(), // map keys need to be comparable
			sb.RandomType(true),
		)
	case 5, 6:
		return PointerOf(sb.RandomType(true))
	case 7:
		return sb.RandStructType(comp)
	case 8:
		return sb.RandFuncType()
	default:
		return RandType()
	}
}

func (sb *StmtBuilder) RandStructType(comparable bool) StructType {
	st := StructType{"ST", []Type{}, []string{}}
	for i := 0; i < sb.rs.Intn(6); i++ {
		t := sb.RandomType(true)
		st.Ftypes = append(st.Ftypes, t)
		st.Fnames = append(st.Fnames, Ident(t)+strconv.Itoa(i))
	}
	return st
}

func (sb *StmtBuilder) RandFuncType() FuncType {
	args := make([]Type, 0, sb.rs.Intn(5))

	// arguments
	for i := 0; i < cap(args); i++ {
		args = append(args, sb.RandomType(true))
	}

	// return type
	ret := []Type{sb.RandomType(true)}

	return FuncType{"FU", args, ret, true}
}

// DeclStmt returns a DeclStmt where nVars new variables of type kind
// are declared, and a list of the newly created *ast.Ident that
// entered the scope.
func (sb *StmtBuilder) DeclStmt(nVars int, t Type) (*ast.DeclStmt, []*ast.Ident) {
	if nVars < 1 {
		panic("nVars < 1")
	}

	if _, ok := t.(FuncType); ok {
		nVars = 1
	}

	gd := new(ast.GenDecl)
	gd.Tok = token.VAR

	// generate the type specifier
	var typ ast.Expr
	var rhs []ast.Expr

	switch t2 := t.(type) {
	case BasicType, ArrayType, PointerType, StructType, ChanType, MapType:
		typ = t2.Ast()

	case FuncType:
		// For function we don't just declare the variable, we also
		// assign to it (so we can give the function a body):
		//
		//  var FNC0 func(int) int = func(p0 int, p1 bool) int {
		//                             Stmts ...
		//                             return <int expr>
		//                           }
		//
		// But 10% of the times we don't (and the func variable will
		// be nil).

		// First off all, remove all the labels currently in scope.
		// The Go Specification says:
		//
		//    The scope of a label is the body of the function in which
		//    it is declared and excludes the body of any nested
		//    function.
		//
		// So the nested function we're about to create cannot use
		// labels created outside its body.
		oldLabels := sb.labels
		sb.labels = []string{}

		// LHS is the type specifier for the given FuncType, with no
		// parameter names
		p, r := t2.MakeFieldLists(false, 0)
		typ = &ast.FuncType{Params: p, Results: r}

		// RHS (with chance 0.9)

		if sb.rs.Intn(10) != 0 {
			// Func type specifier again, but this time with parameter
			// names
			p, r = t2.MakeFieldLists(true, sb.funcp)
			fl := &ast.FuncLit{
				Type: &ast.FuncType{Params: p, Results: r},
				Body: &ast.BlockStmt{},
			}

			// add the parameters to the scope
			for i, param := range fl.Type.Params.List {
				sb.scope.AddVariable(param.Names[0], t2.Args[i])
				sb.funcp++
			}

			// generate a function body
			sb.depth++
			if sb.CanNest() {
				oil := sb.inloop
				sb.inloop = false
				fl.Body = sb.BlockStmt()
				sb.inloop = oil
			} else {
				n := 2 + sb.rs.Intn(3) // 2 to 4 stmts
				stl := make([]ast.Stmt, 0, n)
				for i := 0; i < n; i++ {
					stl = append(stl, sb.AssignStmt())
				}
				fl.Body = &ast.BlockStmt{List: stl}
			}
			sb.depth--

			// Finally, append a closing return statement
			retStmt := &ast.ReturnStmt{Results: []ast.Expr{}}
			for _, ret := range t2.Ret {
				retStmt.Results = append(retStmt.Results, sb.eb.Expr(ret))
			}
			fl.Body.List = append(fl.Body.List, retStmt)
			rhs = append(rhs, fl)

			// remove the function parameters from scope...
			for _, param := range fl.Type.Params.List {
				sb.scope.DeleteIdentByName(param.Names[0])
				sb.funcp--
			}
		}
		// and restore the labels.
		sb.labels = oldLabels

	case TypeParam:
		// for _, tp := range sb.currfunc.Type.TypeParams.List {
		// 	fmt.Println(tp.Type.(*ast.Ident).Name, t2.N.Name, tp.Names[0].Name)
		// 	if tp.Type.(*ast.Ident).Name == t2.N.Name {
		// 		typ = &ast.Ident{
		// 			Name: tp.Names[0].Name,
		// 		}
		// 		break
		// 	}
		// }

		// TODO(alb): this is wrong, we don't declare a variable of
		// the type t passed as an arg to the function we're in.
		typ = &ast.Ident{
			Name: fmt.Sprintf("G%v", 0),
		}

	default:
		panic("DeclStmt bad type " + t.Name())
	}

	// generate nVars ast.Idents
	idents := make([]*ast.Ident, 0, nVars)
	for i := 0; i < nVars; i++ {
		idents = append(idents, sb.scope.NewIdent(t))
	}

	gd.Specs = []ast.Spec{
		&ast.ValueSpec{
			Names:  idents,
			Type:   typ,
			Values: rhs,
		},
	}

	ds := new(ast.DeclStmt)
	ds.Decl = gd

	return ds, idents
}

// Like DeclStmt, but declares a variable of a random TypeParam.
func (sb *StmtBuilder) DeclStmtTypeParam(i int) (*ast.DeclStmt, []*ast.Ident) {
	gd := new(ast.GenDecl)
	gd.Tok = token.VAR
	name := fmt.Sprintf("x%v", i)
	gd.Specs = []ast.Spec{
		&ast.ValueSpec{
			Names:  []*ast.Ident{&ast.Ident{Name: name}},
			Type:   nil,
			Values: nil,
		},
	}

	idents := []*ast.Ident{&ast.Ident{Name: name}}
	ds := new(ast.DeclStmt)
	ds.Decl = gd
	return ds, idents
}

func (sb *StmtBuilder) ForStmt() *ast.ForStmt {

	sb.depth++
	defer func() { sb.depth-- }()

	var fs ast.ForStmt
	// - Cond stmt with chance 0.94 (1-1/16)
	// - Init and Post statements with chance 0.5
	// - A body with chance 0.97 (1-1/32)
	if sb.rs.Intn(16) > 0 {
		fs.Cond = sb.eb.Expr(BasicType{"bool"})
	}
	if sb.rs.Intn(2) > 0 {
		fs.Init = sb.AssignStmt()
	}
	if sb.rs.Intn(2) > 0 {
		fs.Post = sb.AssignStmt()
	}
	if sb.rs.Intn(32) > 0 {
		sb.inloop = true
		fs.Body = sb.BlockStmt()
		sb.inloop = false
	} else {
		// empty loop body
		fs.Body = &ast.BlockStmt{}
	}

	// consume all active labels to avoid unused compilation errors
	for _, l := range sb.labels {
		fs.Body.List = append(fs.Body.List,
			&ast.BranchStmt{
				Tok:   []token.Token{token.GOTO, token.BREAK, token.CONTINUE}[sb.rs.Intn(3)],
				Label: &ast.Ident{Name: l},
			})
	}
	sb.labels = []string{}

	return &fs
}

func (sb *StmtBuilder) RangeStmt(arr Variable) *ast.RangeStmt {

	sb.depth++
	sb.inloop = true
	defer func() { sb.depth--; sb.inloop = false }()

	i := sb.scope.NewIdent(BasicType{"int"})
	var v *ast.Ident
	switch arr.Type.(type) {
	case ArrayType:
		v = sb.scope.NewIdent(arr.Type.(ArrayType).Base())
	case BasicType:
		if arr.Type.(BasicType).N == "string" {
			v = sb.scope.NewIdent(BasicType{"rune"})
		} else {
			panic("cannot range on non-string BasicType")
		}
	default:
		panic("Bad range type")
	}

	rs := &ast.RangeStmt{
		Key:   i,
		Value: v,
		Tok:   token.DEFINE,
		X:     arr.Name,
	}

	rs.Body = sb.BlockStmt()
	rs.Body.List = append(rs.Body.List, sb.UseVars([]*ast.Ident{i, v}))

	sb.scope.DeleteIdentByName(i)
	sb.scope.DeleteIdentByName(v)

	for _, l := range sb.labels {
		rs.Body.List = append(rs.Body.List,
			&ast.BranchStmt{
				Tok:   []token.Token{token.GOTO, token.BREAK, token.CONTINUE}[sb.rs.Intn(3)],
				Label: &ast.Ident{Name: l},
			})
	}
	sb.labels = []string{}

	return rs
}

func (sb *StmtBuilder) DeferStmt() *ast.DeferStmt {
	if v, ok := sb.eb.scope.GetRandomFuncAnyType(); ok && sb.rs.Intn(4) > 0 {
		return &ast.DeferStmt{Call: sb.eb.MakeFuncCall(v)}
	} else {
		return &ast.DeferStmt{Call: sb.eb.CallExpr(RandType(), DEFER)}
	}
}

func (sb *StmtBuilder) IfStmt() *ast.IfStmt {

	sb.depth++
	defer func() { sb.depth-- }()

	is := &ast.IfStmt{
		Cond: sb.eb.Expr(BasicType{"bool"}),
		Body: sb.BlockStmt(),
	}

	// optionally attach an else
	if sb.rs.Intn(2) == 0 {
		is.Else = sb.BlockStmt()
	}

	return is
}

func (sb *StmtBuilder) SwitchStmt() *ast.SwitchStmt {

	sb.depth++
	defer func() { sb.depth-- }()

	t := RandType()
	if sb.rs.Intn(2) == 0 && sb.scope.HasType(PointerOf(t)) {
		// sometimes switch on a pointer value
		t = PointerOf(t)
	}

	ss := &ast.SwitchStmt{
		Tag: sb.eb.Expr(t),
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				// Only generate one normal and one default case to
				// avoid 'duplicate case' compilation errors.
				sb.CaseClause(t, false),
				sb.CaseClause(t, true),
			},
		},
	}

	return ss
}

// builds and returns a single CaseClause switching on type kind. If
// def is true, returns a 'default' switch case.
func (sb *StmtBuilder) CaseClause(t Type, def bool) *ast.CaseClause {
	cc := new(ast.CaseClause)
	if !def {
		cc.List = []ast.Expr{sb.eb.Expr(t)}
	}
	cc.Body = sb.BlockStmt().List
	return cc
}

func (sb *StmtBuilder) IncDecStmt(t Type) *ast.IncDecStmt {
	panic("not implemented")
}

func (sb *StmtBuilder) SendStmt() *ast.SendStmt {
	st := new(ast.SendStmt)

	ch, ok := sb.scope.GetRandomVarChan(sb.rs)
	if !ok {
		// no channels in scope, but we can send to a brand new one,
		// i.e. generate
		//   make(chan int) <- 1
		t := RandType()
		st.Chan = sb.eb.VarOrLit(ChanType{T: t})
		st.Value = sb.eb.Expr(t)
	} else {
		st.Chan = ch.Name
		st.Value = sb.eb.Expr(ch.Type.(ChanType).Base())
	}

	return st
}

func (sb *StmtBuilder) SelectStmt() *ast.SelectStmt {
	sb.depth++
	defer func() { sb.depth-- }()

	return &ast.SelectStmt{
		Body: &ast.BlockStmt{List: []ast.Stmt{
			sb.CommClause(false),
			sb.CommClause(false),
			sb.CommClause(true),
		}},
	}
}

// CommClause is the Select clause. This function returns:
//   case <- [channel]     if def is false
//   default               if def is true
func (sb *StmtBuilder) CommClause(def bool) *ast.CommClause {

	// a couple of Stmt are enough for a select case body
	stmtList := []ast.Stmt{sb.Stmt(), sb.Stmt()}

	if def {
		return &ast.CommClause{Body: stmtList}
	}

	ch, chanInScope := sb.scope.GetRandomVarChan(sb.rs)
	if !chanInScope {
		// when no chan is in scope, we select from a newly made channel,
		// i.e. we build and return
		//    select <-make(chan <random type>)
		t := RandType()
		return &ast.CommClause{
			Comm: &ast.ExprStmt{
				X: &ast.UnaryExpr{
					Op: token.ARROW,
					X: &ast.CallExpr{
						Fun: &ast.Ident{Name: "make"},
						Args: []ast.Expr{
							&ast.ChanType{
								Dir:   3,
								Value: t.Ast(),
							},
						},
					},
				},
			},
			Body: stmtList,
		}
	}

	// otherwise, we receive from one of the channels in scope
	return &ast.CommClause{
		Comm: &ast.ExprStmt{
			X: sb.eb.ChanReceiveExpr(ch.Name),
		},
		Body: stmtList,
	}

}

func (sb *StmtBuilder) ExprStmt() *ast.ExprStmt {
	if ch, ok := sb.scope.GetRandomVarChan(sb.rs); ok {
		return &ast.ExprStmt{
			X: sb.eb.ChanReceiveExpr(ch.Name),
		}
	}

	// len() is not allowed (it's not really a function), DEFER here
	// prevents CallExpr to choose len as the function to call.
	return &ast.ExprStmt{sb.eb.CallExpr(RandType(), DEFER)}
}

var noName = ast.Ident{Name: "_"}

// build and return a statement of form
//   _, _, ... _ = var1, var2, ..., varN
// for each i in idents
func (sb *StmtBuilder) UseVars(idents []*ast.Ident) ast.Stmt {
	useStmt := &ast.AssignStmt{
		Lhs: make([]ast.Expr, 0, len(idents)),
		Tok: token.ASSIGN,
		Rhs: make([]ast.Expr, 0, len(idents)),
	}

	for _, name := range idents {
		useStmt.Lhs = append(useStmt.Lhs, &noName)
		useStmt.Rhs = append(useStmt.Rhs, name)
	}
	return useStmt
}
