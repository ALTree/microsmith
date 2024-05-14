package microsmith

import (
	"fmt"
	"go/ast"
	"go/token"
	"math/rand"
)

// --------------------------------
//  StmtBuilder type
// --------------------------------

type StmtBuilder struct {
	pb *PackageBuilder

	C *Context
	E *ExprBuilder
	R *rand.Rand
	S *Scope

	// TODO(alb): move all of these into Context or PackageBuilder
	depth  int // how deep the stmt hyerarchy is
	funcp  int // counter for function param names
	labels []string
	label  int // counter for labels names
}

func NewStmtBuilder(pb *PackageBuilder) *StmtBuilder {
	return &StmtBuilder{
		pb: pb,
		C:  pb.ctx,
		E:  nil, // this hasn't been created yet
		R:  pb.rs,
		S:  pb.ctx.scope,
	}
}

// --------------------------------
//  Builder Methods
// --------------------------------

// Returns true if the block statement currently being built is
// allowed to have statements nested inside it.
func (sb *StmtBuilder) CanNest() bool {
	return (sb.depth <= 3) && (sb.R.Float64() < 0.8)
}

func (sb *StmtBuilder) Stmt() ast.Stmt {
	if !sb.CanNest() {
		return sb.AssignStmt()
	}

	switch sb.R.Intn(12) {
	case 0:
		return sb.AssignStmt()
	case 1:
		return sb.BlockStmt()
	case 2:
		if sb.R.Intn(2) == 0 { // for range
			return sb.RangeStmt()
		}
		if sb.R.Intn(4) == 0 { // labelled plain for
			sb.label++
			label := fmt.Sprintf("lab%v", sb.label)
			sb.labels = append(sb.labels, label)
			fs := &ast.LabeledStmt{
				Label: &ast.Ident{Name: label},
				Stmt:  sb.ForStmt(),
			}
			return fs
		}
		return sb.ForStmt() // plain for
	case 3:
		return sb.IfStmt()
	case 4:
		return sb.SwitchStmt()
	case 5:
		return sb.SendStmt()
	case 6:
		return sb.SelectStmt()
	case 7:
		if sb.C.inLoop {
			return sb.BranchStmt()
		}
		return sb.AssignStmt()
	case 8:
		return sb.DeferStmt()
	case 9:
		return sb.GoStmt()
	case 10:
		return sb.ExprStmt()
	case 11:
		return sb.ClearStmt()
	default:
		panic("unreachable")
	}
}

// gets a random variable currently in scope (that we can assign to),
// and builds an AssignStmt with a random Expr of its type on the RHS
func (sb *StmtBuilder) AssignStmt() *ast.AssignStmt {
	v, ok := sb.S.RandAssignable()
	if !ok {
		fmt.Println(sb.S)
		panic("No assignable variable in scope")
	}

	switch t := v.Type.(type) {

	case StructType:
		// For structs, 50/50 between assigning to the variable and
		// setting one of its fields.
		if sb.R.Intn(2) == 0 || len(t.Ftypes) == 0 {
			// v = struct{<expr>, <expr>, ...}
			return &ast.AssignStmt{
				Lhs: []ast.Expr{v.Name},
				Tok: token.ASSIGN,
				Rhs: []ast.Expr{sb.E.CompositeLit(t)},
			}
		} else {
			// v.field = <expr>
			//
			// we need to avoid getting to a chan receive here, i.e. we
			// need to avoid generating:
			//
			//   <-st.c = ...
			//
			// for a st struct{ c chan int } (at any depth), because
			// that will fail to compile with error:
			//
			//   cannot assign to <-st.c (comma, ok expression of type int)
			//
			// So, only in AssignStmt, never go deeper than 1 level
			// inside structs, and assign directly to a depth-1 field
			// (that is not a chan).
			//
			// TODO(alb): this can be changed to go to arbitrary-depth
			// as long as it avoids channels. Doable?
			fis := make([]int, 0, len(t.Ftypes))
			for i, ft := range t.Ftypes {
				if _, ok := ft.(ChanType); !ok {
					fis = append(fis, i)
				}
			}

			if len(fis) == 0 { // fallback to v = struct{ ... }
				return &ast.AssignStmt{
					Lhs: []ast.Expr{v.Name},
					Tok: token.ASSIGN,
					Rhs: []ast.Expr{sb.E.CompositeLit(t)},
				}
			}

			fi := RandItem(sb.pb.rs, fis)
			return &ast.AssignStmt{
				Lhs: []ast.Expr{&ast.SelectorExpr{X: v.Name, Sel: &ast.Ident{Name: t.Fnames[fi]}}},
				Tok: token.ASSIGN,
				Rhs: []ast.Expr{sb.E.Expr(t.Ftypes[fi])},
			}
		}

	case ArrayType:
		// For arrays, 50/50 between
		//   A[<expr>] = <expr>
		//   A = { <expr>, <expr>, ... }
		if sb.R.Intn(2) == 0 {
			return &ast.AssignStmt{
				Lhs: []ast.Expr{sb.E.IndexExpr(v.Name)},
				Tok: token.ASSIGN,
				Rhs: []ast.Expr{sb.E.Expr(t.Base())},
			}
		} else {
			return &ast.AssignStmt{
				Lhs: []ast.Expr{v.Name},
				Tok: token.ASSIGN,
				Rhs: []ast.Expr{sb.E.Expr(v.Type)},
			}
		}

	case MapType:
		// For maps, 50/50 between
		//   M[<expr>] = <expr>
		//   M = { <expr>: <expr> }
		if sb.R.Intn(2) == 0 {
			return &ast.AssignStmt{
				Lhs: []ast.Expr{sb.E.MapIndexExpr(v.Name, v.Type.(MapType).KeyT)},
				Tok: token.ASSIGN,
				Rhs: []ast.Expr{sb.E.Expr(v.Type.(MapType).ValueT)},
			}
		} else {
			return &ast.AssignStmt{
				Lhs: []ast.Expr{v.Name},
				Tok: token.ASSIGN,
				Rhs: []ast.Expr{sb.E.Expr(v.Type)},
			}
		}

	default:
		return &ast.AssignStmt{
			Lhs: []ast.Expr{v.Name},
			Tok: token.ASSIGN,
			Rhs: []ast.Expr{sb.E.Expr(v.Type)},
		}
	}
}

// returns a continue/break statement
func (sb *StmtBuilder) BranchStmt() *ast.BranchStmt {
	var bs ast.BranchStmt

	bs.Tok = RandItem(sb.R, []token.Token{token.GOTO, token.CONTINUE, token.BREAK})

	// break/continue/goto to a label with chance 0.25
	if len(sb.labels) > 0 && sb.R.Intn(4) == 0 {
		li := sb.R.Intn(len(sb.labels))
		bs.Label = &ast.Ident{Name: sb.labels[li]}
		sb.labels = append(sb.labels[:li], sb.labels[li+1:]...)
	} else {
		// If we didn't add a label, GOTO is not allowed.
		if sb.R.Intn(2) == 0 {
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
	for _, t := range sb.pb.RandTypes(3 + sb.R.Intn(6)) {
		newDecl, nv := sb.DeclStmt(1+sb.R.Intn(3), t)
		stmts = append(stmts, newDecl)
		newVars = append(newVars, nv...)
	}

	var nStmts int
	if !sb.CanNest() {
		// If we stop nesting statements, guarantee 8 assignmens, to
		// so we don't generate almost-empty blocks.
		nStmts = 8
	} else {
		nStmts = 4 + sb.R.Intn(5)
	}

	// Fill the block's body.
	for i := 0; i < nStmts; i++ {
		stmts = append(stmts, sb.Stmt())
	}

	if len(newVars) > 0 {
		stmts = append(stmts, sb.UseVars(newVars))
	}

	for _, v := range newVars {
		sb.S.DeleteIdentByName(v)
	}

	bs.List = stmts
	return bs
}

// FuncBody returns a BlockStmt, like BlockStmt, except it appends a
// ReturnStmt of the given types at the end.
func (sb *StmtBuilder) FuncBody(t []Type) *ast.BlockStmt {
	b := sb.BlockStmt()
	if len(t) > 0 {
		b.List = append(b.List, sb.ReturnStmt(t))
	}
	return b
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
	case BasicType, ArrayType, PointerType, StructType, ChanType, MapType, InterfaceType:
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

		if sb.R.Intn(10) != 0 {
			// Func type specifier again, but this time with parameter
			// names
			p, r = t2.MakeFieldLists(true, sb.funcp)
			fl := &ast.FuncLit{
				Type: &ast.FuncType{Params: p, Results: r},
				Body: &ast.BlockStmt{},
			}

			// add the parameters to the scope
			for i, param := range p.List {
				if ep, ok := t2.Args[i].(EllipsisType); ok {
					sb.S.AddVariable(param.Names[0], ArrayOf(ep.Base))
				} else {
					sb.S.AddVariable(param.Names[0], t2.Args[i])
				}

				sb.funcp++
			}

			// generate a function body
			sb.depth++
			if sb.CanNest() {
				old := sb.C.inLoop
				sb.C.inLoop = false
				defer func() { sb.C.inLoop = old }()
				fl.Body = sb.BlockStmt()
			} else {
				n := 2 + sb.R.Intn(3)
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
				retStmt.Results = append(retStmt.Results, sb.E.Expr(ret))
			}
			fl.Body.List = append(fl.Body.List, retStmt)
			rhs = append(rhs, fl)

			// remove the function parameters from scope...
			for _, param := range fl.Type.Params.List {
				sb.S.DeleteIdentByName(param.Names[0])
				sb.funcp--
			}
		}
		// and restore the labels.
		sb.labels = oldLabels

	case TypeParam:
		typ = t2.Ast()

	default:
		panic("DeclStmt bad type " + t.Name())
	}

	idents := make([]*ast.Ident, 0, nVars)
	for i := 0; i < nVars; i++ {
		idents = append(idents, sb.S.NewIdent(t))
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
	if sb.R.Intn(16) > 0 {
		fs.Cond = sb.E.Expr(BT{"bool"})
	}
	if sb.R.Intn(2) > 0 {
		fs.Init = sb.AssignStmt()
	}
	if sb.R.Intn(2) > 0 {
		fs.Post = sb.AssignStmt()
	}
	if sb.R.Intn(32) > 0 {
		old := sb.C.inLoop
		sb.C.inLoop = false
		defer func() { sb.C.inLoop = old }()
		fs.Body = sb.BlockStmt()
	} else {
		// empty loop body
		fs.Body = &ast.BlockStmt{}
	}

	// consume all active labels to avoid unused compilation errors
	for _, l := range sb.labels {
		fs.Body.List = append(fs.Body.List,
			&ast.BranchStmt{
				Tok:   RandItem(sb.R, []token.Token{token.GOTO, token.BREAK, token.CONTINUE}),
				Label: &ast.Ident{Name: l},
			})
	}
	sb.labels = []string{}

	return &fs
}

func (sb *StmtBuilder) RangeStmt() *ast.RangeStmt {
	sb.depth++
	old := sb.C.inLoop
	sb.C.inLoop = true
	defer func() { sb.depth--; sb.C.inLoop = old }()

	// it's either
	//   k := range [int]
	// or
	//   k, v := range [string or slice]
	// or
	//  [k, v] := range [function]

	var k, v *ast.Ident
	var e ast.Expr

	f := sb.E.Expr
	if !sb.E.Deepen() {
		f = sb.E.VarOrLit
	}

	// randomly choose a type for the expression we range on
	switch sb.R.Intn(4) {
	case 0: // slice
		t := ArrayOf(sb.pb.RandType())
		e = f(t)
		k = sb.S.NewIdent(BT{"int"})
		v = sb.S.NewIdent(t.Base())
	case 1: // string
		e = f(BT{"string"})
		k = sb.S.NewIdent(BT{"int"})
		v = sb.S.NewIdent(BT{"rune"})
	case 2: // int
		e = f(BT{"int"})
		k = sb.S.NewIdent(BT{"int"})
	case 3: // func
		ft := sb.pb.RandRangeableFuncType()
		p, r := ft.MakeFieldLists(true, sb.funcp)

		// add yield param to the scope
		sb.S.AddVariable(p.List[0].Names[0], ft.Args[0])
		sb.funcp++

		// generate a body for the func
		sb.depth++
		var body *ast.BlockStmt
		if sb.CanNest() {
			old := sb.C.inLoop
			sb.C.inLoop = false
			body = sb.BlockStmt()
			sb.C.inLoop = old
		} else {
			body = &ast.BlockStmt{List: []ast.Stmt{sb.AssignStmt()}}
		}
		sb.depth--

		e = &ast.FuncLit{
			Type: &ast.FuncType{Params: p, Results: r},
			Body: body,
		}

		// remove the yield param from the scope
		sb.S.DeleteIdentByName(p.List[0].Names[0])
		sb.funcp--

		// declare the iteration variables if needed
		switch args := ft.Args[0].(FuncType).Args; len(args) {
		case 1:
			k = sb.S.NewIdent(args[0])
		case 2:
			k = sb.S.NewIdent(args[0])
			v = sb.S.NewIdent(args[1])
		}
	default:
		panic("unreachable")
	}

	rs := &ast.RangeStmt{Tok: token.DEFINE, X: e, Body: sb.BlockStmt()}

	if k != nil {
		rs.Key = k
		rs.Body.List = append(rs.Body.List, sb.UseVars([]*ast.Ident{k}))
		sb.S.DeleteIdentByName(k)
	}

	if v != nil {
		rs.Value = v
		rs.Body.List = append(rs.Body.List, sb.UseVars([]*ast.Ident{v}))
		sb.S.DeleteIdentByName(v)
	}

	return rs
}

func (sb *StmtBuilder) DeferStmt() *ast.DeferStmt {
	if v, ok := sb.S.RandFunc(); ok && sb.R.Intn(4) > 0 {
		return &ast.DeferStmt{Call: sb.E.CallFunction(v)}
	} else {
		old := sb.C.inDefer
		sb.C.inDefer = true
		defer func() { sb.C.inDefer = old }()
		return &ast.DeferStmt{Call: sb.E.ConjureAndCallFunc(sb.pb.RandType())}
	}
}

func (sb *StmtBuilder) GoStmt() *ast.GoStmt {
	if v, ok := sb.S.RandFunc(); ok && sb.R.Intn(4) > 0 {
		return &ast.GoStmt{Call: sb.E.CallFunction(v)}
	} else {
		old := sb.C.inDefer
		sb.C.inDefer = true
		defer func() { sb.C.inDefer = old }()
		return &ast.GoStmt{Call: sb.E.ConjureAndCallFunc(sb.pb.RandType())}
	}
}

func (sb *StmtBuilder) IfStmt() *ast.IfStmt {

	sb.depth++
	defer func() { sb.depth-- }()

	is := &ast.IfStmt{
		Cond: sb.E.Expr(BT{"bool"}),
		Body: sb.BlockStmt(),
	}

	// optionally attach an else
	if sb.R.Intn(2) == 0 {
		is.Else = sb.BlockStmt()
	}

	return is
}

// ReturnStmt builds a return statement with expression of the given
// types.
func (sb *StmtBuilder) ReturnStmt(types []Type) *ast.ReturnStmt {
	sb.depth++
	defer func() { sb.depth-- }()

	ret := &ast.ReturnStmt{Results: make([]ast.Expr, len(types))}

	for i, t := range types {
		ret.Results[i] = sb.E.Expr(t)
	}

	return ret

}

func (sb *StmtBuilder) SwitchStmt() *ast.SwitchStmt {
	sb.depth++
	defer func() { sb.depth-- }()

	t := sb.pb.RandComparableType()
	if sb.R.Intn(2) == 0 && sb.S.Has(PointerOf(t)) {
		// sometimes switch on a pointer value
		t = PointerOf(t)
	}

	ss := &ast.SwitchStmt{
		Tag:  sb.E.Expr(t),
		Body: &ast.BlockStmt{List: []ast.Stmt{}},
	}

	// add a few cases
	for i := 0; i < sb.R.Intn(4); i++ {
		cc, ok := sb.CaseClause(t, false)
		ss.Body.List = append(ss.Body.List, cc)
		if !ok {
			break
		}
	}

	// optionally add a default case
	if sb.R.Intn(3) != 0 {
		cc, _ := sb.CaseClause(t, true)
		ss.Body.List = append(ss.Body.List, cc)
	}
	return ss
}

// builds and returns a single CaseClause switching on type kind. If
// def is true, returns a 'default' switch case.
func (sb *StmtBuilder) CaseClause(t Type, def bool) (*ast.CaseClause, bool) {
	cc := new(ast.CaseClause)
	ret := true
	if !def {
		e, ok := sb.E.NonConstantExpr(t)
		if !ok {
			ret = false
		}
		cc.List = []ast.Expr{e}
	}
	cc.Body = sb.BlockStmt().List
	return cc, ret
}

func (sb *StmtBuilder) IncDecStmt(t Type) *ast.IncDecStmt {
	panic("not implemented")
}

func (sb *StmtBuilder) SendStmt() *ast.SendStmt {
	st := new(ast.SendStmt)
	if ch, ok := sb.S.RandChan(); !ok {
		t := sb.pb.RandType()
		st.Chan = sb.E.VarOrLit(ChanType{T: t})
		st.Value = sb.E.Expr(t)
	} else {
		st.Chan = ch.Name
		st.Value = sb.E.Expr(ch.Type.(ChanType).Base())
	}

	return st
}

func (sb *StmtBuilder) SelectStmt() *ast.SelectStmt {
	sb.depth++
	defer func() { sb.depth-- }()

	ss := &ast.SelectStmt{
		Body: &ast.BlockStmt{List: []ast.Stmt{}},
	}

	for i := 0; i < sb.R.Intn(4); i++ {
		ss.Body.List = append(ss.Body.List, sb.CommClause(false))
	}

	if sb.R.Intn(4) == 0 {
		ss.Body.List = append(ss.Body.List, sb.CommClause(true))
	}

	return ss
}

// CommClause is the Select clause. This function returns:
//
//	case <-c        if def is false
//	default         if def is true
func (sb *StmtBuilder) CommClause(def bool) *ast.CommClause {

	// a couple of Stmt are enough for a select case body
	stmtList := []ast.Stmt{sb.Stmt(), sb.Stmt()}

	if def {
		return &ast.CommClause{Body: stmtList}
	}

	ch, chanInScope := sb.S.RandChan()
	if !chanInScope {
		// when no chan is in scope, we select from a newly made channel,
		// i.e. we build and return
		//    select <-make(chan <random type>)
		t := sb.pb.RandType()
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
			X: sb.E.ChanReceiveExpr(ch.Name),
		},
		Body: stmtList,
	}

}

func (sb *StmtBuilder) ExprStmt() *ast.ExprStmt {

	// Close(ch) or <-ch.
	if ch, ok := sb.S.RandChan(); ok && sb.R.Intn(4) == 0 {
		if sb.R.Intn(2) == 0 {
			return &ast.ExprStmt{
				X: sb.E.ChanReceiveExpr(ch.Name),
			}
		} else {
			return &ast.ExprStmt{
				X: &ast.CallExpr{
					Fun:  CloseIdent,
					Args: []ast.Expr{ch.Name},
				},
			}
		}
	}

	// Call a random function. We don't use RandCallExpr() because
	// that could choose a built-in (like len), which is not allowed
	// as an ExprStmt. Conjuring a new function and calling it will
	// always work.
	return &ast.ExprStmt{sb.E.ConjureAndCallFunc(sb.pb.RandType())}
}

func (sb *StmtBuilder) ClearStmt() *ast.ExprStmt {

	var arg ast.Expr
	if rn, ok := sb.S.RandClearable(); ok && sb.R.Intn(3) < 2 {
		arg = rn.Name
	} else {
		if sb.R.Intn(2) == 0 {
			arg = sb.E.MakeMakeCall(ArrayOf(sb.pb.RandType()))
		} else {
			arg = sb.E.MakeMakeCall(MapOf(sb.pb.RandComparableType(), sb.pb.RandType()))
		}
	}

	return &ast.ExprStmt{
		X: &ast.CallExpr{
			Fun:  ClearIdent,
			Args: []ast.Expr{arg},
		},
	}
}

var noName = ast.Ident{Name: "_"}

// build and return a statement of form
//
//	_, _, ... _ = var1, var2, ..., varN
//
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
