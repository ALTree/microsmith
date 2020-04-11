package microsmith

import (
	"go/ast"
	"go/token"
	"math/rand"
)

type StmtBuilder struct {
	rs     *rand.Rand // randomness source
	eb     *ExprBuilder
	depth  int // how deep the stmt hyerarchy is
	conf   ProgramConf
	scope  *Scope
	inloop bool // are we inside a loop?
	funcp  int  // counter for function param names
	stats  StmtStats
}

type StmtStats struct {
	Assign int
	Branch int
	Block  int
	For    int
	If     int
	Switch int
	Send   int
	Select int
}

type StmtConf struct {
	MaxStmtDepth int // max depth of block nesting
}

func NewStmtBuilder(rs *rand.Rand, conf ProgramConf) *StmtBuilder {
	sb := new(StmtBuilder)
	sb.rs = rs
	sb.conf = conf

	scope := make(Scope, 0, 16)

	// pre-declared function are always in scope
	scope = append(scope, Variable{
		LenFun,
		&ast.Ident{Name: LenFun.Name()}})
	scope = append(scope, Variable{
		FloatConv,
		&ast.Ident{Name: FloatConv.Name()}})

	// a few functions from the math package to call
	scope = append(scope, Variable{
		MathSqrt,
		&ast.Ident{Name: MathSqrt.Name()}})
	scope = append(scope, Variable{
		MathMax,
		&ast.Ident{Name: MathMax.Name()}})
	scope = append(scope, Variable{
		MathNaN,
		&ast.Ident{Name: MathNaN.Name()}})

	sb.scope = &scope
	sb.eb = NewExprBuilder(rs, conf, sb.scope)

	return sb
}

func (sb *StmtBuilder) Stmt() ast.Stmt {
	// If the maximum allowed stmt depth has been reached, generate an
	// assign statement, since they don't nest.
	if sb.depth > sb.conf.MaxStmtDepth {
		return sb.AssignStmt()
	}
	// sb.depth < sb.conf.MaxStmtDepth

	switch sb.rs.Intn(7) {
	case 0:
		// Generate a BranchStmt (break/continue) instead of an
		// assignment, with chance 0.25, but only if inside a loop.
		if sb.inloop && sb.rs.Intn(4) == 0 {
			return sb.BranchStmt()
		} else {
			return sb.AssignStmt()
		}
	case 1:
		return sb.BlockStmt()
	case 2:
		// If at least one array or string is in scope, generate a for
		// range loop with chance 0.5; otherwise generate a plain loop
		arr, ok := sb.scope.GetRandomRangeable(sb.rs)
		if ok && sb.rs.Intn(2) == 0 {
			return sb.RangeStmt(arr)
		} else {
			return sb.ForStmt()
		}
	case 3:
		return sb.IfStmt()
	case 4:
		return sb.SwitchStmt()
	case 5:
		return sb.SendStmt()
	case 6:
		return sb.SelectStmt()
	default:
		panic("Stmt: bad index")
	}
}

// gets a random variable currently in scope (that we can assign to),
// and builds an AssignStmt with a random Expr of its type on the RHS
func (sb *StmtBuilder) AssignStmt() *ast.AssignStmt {
	sb.stats.Assign++

	v := sb.scope.RandomVar(true)

	switch t := v.Type.(type) {

	case StructType:
		// we don't build struct literals yet, so if we got a struct
		// we'll assign to one of its fields.
		//
		// TODO(alb): implement support for struct literals in Expr,
		// then enable assignments to structs
		fieldType := t.Ftypes[sb.rs.Intn(len(t.Fnames))]
		return &ast.AssignStmt{
			// struct.field = <expr>
			Lhs: []ast.Expr{sb.eb.StructFieldExpr(v, fieldType)},
			Tok: token.ASSIGN,
			Rhs: []ast.Expr{sb.eb.Expr(fieldType)},
		}

	case ArrayType:
		// if we got an array, 50/50 between
		//   1. AI = []int{ <exprs> }
		//   2. AI[<expr>] = <expr>
		if sb.rs.Intn(2) == 0 {
			return &ast.AssignStmt{
				Lhs: []ast.Expr{sb.eb.ArrayIndexExpr(v)},
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
		return &ast.AssignStmt{
			Lhs: []ast.Expr{sb.eb.MapIndexExpr(v)},
			Tok: token.ASSIGN,
			Rhs: []ast.Expr{sb.eb.Expr(v.Type.(MapType).ValueT)},
		}

	case FuncType:
		// TODO(alb): re-enable assigning to FuncType variables? We
		// don't do that now since we already give them a body in
		// their DeclStmt, but it could be interesting to re-assign
		// then a new body in the program.
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
	sb.stats.Branch++
	if sb.rs.Intn(2) == 0 {
		return &ast.BranchStmt{
			Tok: token.CONTINUE,
		}
	} else {
		return &ast.BranchStmt{
			Tok: token.BREAK,
		}
	}
}

// BlockStmt returns a new Block Statement. The returned Stmt is
// always a valid block. It up to BlockStmt's caller to make sure
// BlockStmt is only called when we have not yet reached max depth.
func (sb *StmtBuilder) BlockStmt() *ast.BlockStmt {

	sb.depth++
	defer func() { sb.depth-- }()

	sb.stats.Block++

	bs := new(ast.BlockStmt)
	stmts := []ast.Stmt{}

	// A new block means opening a new scope.
	//
	// Start with nDecls line of variables declarations, each of a
	// different random type.
	nDecls := 3 + sb.rs.Intn(6)
	var newVars []*ast.Ident
	for _, t := range sb.RandomTypes(nDecls) {
		newDecl, nv := sb.DeclStmt(1+sb.rs.Intn(3), t)
		stmts = append(stmts, newDecl)
		newVars = append(newVars, nv...)
	}

	var nStmts int
	if sb.depth == sb.conf.MaxStmtDepth {
		// At maxdepth, only assignments are allowed. Guarantee 8 of
		// them, to be sure not to generate almost-empty blocks.
		nStmts = 8
	} else {
		nStmts = 4 + sb.rs.Intn(5)
	}

	// Fill the block's body.
	for i := 0; i < nStmts; i++ {
		stmts = append(stmts, sb.Stmt())
	}

	if len(newVars) > 0 {
		stmts = append(stmts, sb.UseVars(newVars))
	}

	// ...and then remove them from scope.
	for _, v := range newVars {
		sb.scope.DeleteIdentByName(v)
	}

	bs.List = stmts
	return bs
}

// Returns an arrays of n random Types. That can include basic types,
// arrays, chans, and randomly generated struct types. It's guaranteed
// to have at least one addressable/assignable type.
func (sb *StmtBuilder) RandomTypes(n int) []Type {
	if n < 1 {
		panic("RandomTypes: n is 0")
	}

	types := make([]Type, 0, n)

	// first, our mandatory assignable type; use an Int
	types = append(types, BasicType{"int"})
	n--

	st := sb.conf.SupportedTypes
	for ; n > 0; n-- {
		// Choose at random between a struct, a function, or a basic
		// type with chances 1, 1, 4
		switch sb.rs.Intn(6) {
		case 0:
			types = append(types, RandStructType(st))
		case 1:
			types = append(types, RandFuncType(st))
		default:
			t := RandType(st)
			switch sb.rs.Intn(6) {
			case 0:
				t = ArrOf(t)
			case 1:
				t2 := RandType(st)
				t = MapOf(t, t2)
			case 2:
				t = PointerOf(t)
			case 3:
				t = ChanOf(t)
			default: // 3 < n < 6
				// 1/3 of keeping t as a plain variable
			}
			types = append(types, t)
		}
	}

	return types
}

// DeclStmt returns a DeclStmt where nVars new variables of type kind
// are declared, and a list of the newly created *ast.Ident that
// entered the scope.
func (sb *StmtBuilder) DeclStmt(nVars int, t Type) (*ast.DeclStmt, []*ast.Ident) {
	if nVars < 1 {
		panic("DeclStmt: nVars < 1")
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
	case BasicType:
		typ = TypeIdent(t2.Name())
	case ArrayType:
		typ = &ast.ArrayType{Elt: TypeIdent(t2.Base().Name())}
	case PointerType:
		typ = &ast.StarExpr{X: TypeIdent(t2.Base().Name())}
	case StructType:
		typ = t2.BuildAst()
	case ChanType:
		typ = &ast.ChanType{Dir: 3, Value: TypeIdent(t2.Base().Name())}
	case MapType:
		typ = &ast.MapType{
			Key:   TypeIdent(t2.KeyT.Name()),
			Value: TypeIdent(t2.ValueT.Name()),
		}
	case FuncType:
		// For function we don't just declare the variable, we also
		// assign to it (so we can give the function a body):
		//
		//  var FNC0 func(int) int = func(p0 int, p1 bool) int {
		//                             Stmt
		//                             Stmt
		//                             return <int expr>
		//                           }
		//

		// LHS is the type specifier for the given FuncType, with no
		// parameter names
		p, r := t2.MakeFieldLists(false, 0)
		typ = &ast.FuncType{Params: p, Results: r}

		// RHS

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
		if sb.depth < sb.conf.MaxStmtDepth {
			oldInloop := sb.inloop
			sb.inloop = false
			fl.Body = sb.BlockStmt()
			sb.inloop = oldInloop
		} else {
			fl.Body = &ast.BlockStmt{
				List: []ast.Stmt{
					sb.AssignStmt(),
					sb.AssignStmt(),
				},
			}
		}
		sb.depth--

		// Finally, append a closing return statement.
		retStmt := &ast.ReturnStmt{Results: []ast.Expr{}}
		for _, ret := range t2.Ret {
			retStmt.Results = append(retStmt.Results, sb.eb.Expr(ret))
		}
		fl.Body.List = append(fl.Body.List, retStmt)
		rhs = append(rhs, fl)

		// and remove the function parameters from the scope
		for _, param := range fl.Type.Params.List {
			sb.scope.DeleteIdentByName(param.Names[0])
			sb.funcp--
		}

	default:
		panic("DeclStmt: bad type " + t.Name())
	}

	// generate nVars ast.Idents
	idents := make([]*ast.Ident, 0, nVars)
	for i := 0; i < nVars; i++ {
		idents = append(idents, sb.scope.NewIdent(t))
	}

	gd.Specs = []ast.Spec{
		&ast.ValueSpec{
			Names:  idents,
			Type:   typ.(ast.Expr),
			Values: rhs,
		},
	}

	ds := new(ast.DeclStmt)
	ds.Decl = gd

	return ds, idents
}

func (sb *StmtBuilder) ForStmt() *ast.ForStmt {

	sb.depth++
	defer func() { sb.depth-- }()
	sb.stats.For++

	var fs ast.ForStmt
	// - Cond stmt with chance 0.94 (1-1/16)
	// - Init and Post statements with chance 0.5
	// - A body with chance 0.97 (1-1/32)
	if sb.rs.Int63()%16 != 0 {
		fs.Cond = sb.eb.Expr(BasicType{"bool"})
	}
	if sb.rs.Int63()%2 == 0 {
		fs.Init = sb.AssignStmt()
	}
	if sb.rs.Int63()%2 == 0 {
		fs.Post = sb.AssignStmt()
	}
	if sb.rs.Int63()%32 != 0 {
		sb.inloop = true
		fs.Body = sb.BlockStmt()
		sb.inloop = false
	} else {
		// empty loop body
		fs.Body = &ast.BlockStmt{}
	}

	return &fs
}

func (sb *StmtBuilder) RangeStmt(arr Variable) *ast.RangeStmt {

	sb.depth++
	defer func() { sb.depth-- }()
	sb.inloop = true
	defer func() { sb.inloop = false }()
	sb.stats.For++

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

	return rs
}

func (sb *StmtBuilder) IfStmt() *ast.IfStmt {

	sb.depth++
	defer func() { sb.depth-- }()
	sb.stats.If++

	is := &ast.IfStmt{
		Cond: sb.eb.Expr(BasicType{"bool"}),
		Body: sb.BlockStmt(),
	}

	// optionally attach an 'else'
	if sb.rs.Intn(2) == 0 {
		is.Else = sb.BlockStmt()
	}

	return is
}

func (sb *StmtBuilder) SwitchStmt() *ast.SwitchStmt {

	sb.depth++
	defer func() { sb.depth-- }()
	sb.stats.Switch++

	t := RandType(sb.conf.SupportedTypes)
	if sb.rs.Intn(2) == 0 && sb.scope.HasType(PointerOf(t)) {
		// sometimes switch on a pointer value
		t = PointerOf(t)
	}

	ss := &ast.SwitchStmt{
		Tag: sb.eb.Expr(t),
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				// only generate one normal and one default case to
				// avoid 'duplicate case' compilation errors
				sb.CaseClause(t, false),
				sb.CaseClause(t, true), // 'default:'
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

	sb.stats.Send++

	st := new(ast.SendStmt)
	ch, ok := sb.scope.GetRandomVarChan(sb.rs)
	if !ok {
		// no channels in scope, but we can send to a brand new one,
		// i.e. generate
		//   make(chan int) <- 1
		t := RandType(sb.conf.SupportedTypes)
		st.Chan = &ast.CallExpr{
			Fun: &ast.Ident{Name: "make"},
			Args: []ast.Expr{
				&ast.ChanType{
					Dir:   3,
					Value: TypeIdent(t.Name()),
				},
			},
		}
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
	sb.stats.Select++

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
		t := RandType(sb.conf.SupportedTypes)
		return &ast.CommClause{
			Comm: &ast.ExprStmt{
				X: &ast.UnaryExpr{
					Op: token.ARROW,
					X: &ast.CallExpr{
						Fun: &ast.Ident{Name: "make"},
						Args: []ast.Expr{
							&ast.ChanType{
								Dir:   3,
								Value: TypeIdent(t.Name()),
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
			X: sb.eb.ChanReceiveExpr(ch),
		},
		Body: stmtList,
	}

}

// What is allowed, according to the spec:
//   - function and method calls
//   - receive operation
//
// Currently disabled because the code is a mess and it doesn't add
// much to the generated programs anyway.
func (sb *StmtBuilder) ExprStmt() *ast.ExprStmt {
	panic("not implemented")
}

var noName = ast.Ident{Name: "_"}

// build and return a statement of form
//   _, _, ... _ = var1, var2, ..., varN
// for each ident in idents
func (sb *StmtBuilder) UseVars(idents []*ast.Ident) ast.Stmt {
	useStmt := &ast.AssignStmt{Tok: token.ASSIGN}
	for _, name := range idents {
		useStmt.Lhs = append(useStmt.Lhs, &noName)
		useStmt.Rhs = append(useStmt.Rhs, name)
	}
	return useStmt
}
