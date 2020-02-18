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

	// TODO: comment
	stats StmtStats
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
	// maximum allowed blocks depth
	MaxStmtDepth int

	// max amount of variables and statements inside new block
	MaxBlockVars  int
	MaxBlockStmts int
}

func NewStmtBuilder(rs *rand.Rand, conf ProgramConf) *StmtBuilder {
	sb := new(StmtBuilder)
	sb.rs = rs
	sb.conf = conf

	scope := make(Scope, 0)

	// pre-declared function are always in scope
	scope = append(scope, Variable{
		LenFun,
		&ast.Ident{Name: LenFun.Name()}})
	scope = append(scope, Variable{
		FloatConv,
		&ast.Ident{Name: FloatConv.Name()}})

	// a couple int and float64 functions from the math package
	scope = append(scope, Variable{
		MathSqrt,
		&ast.Ident{Name: MathSqrt.Name()}})
	scope = append(scope, Variable{
		MathMax,
		&ast.Ident{Name: MathMax.Name()}})

	sb.scope = &scope
	sb.eb = NewExprBuilder(rs, conf, sb.scope)

	return sb
}

// ---------------- //
//       Stmt       //
// ---------------- //

// Currently we generate:
//   - AssignStmt
//   - BlockStmt
//   - ForStmt
//   - IfStmt
//   - SwitchStmt
//   - IncDecStmt
//   - SendStmt
//
// DeclStmt is implemented and used, but is not used directly here (by
// Stmt. It's only used inside BlockStmt, which takes care of setting
// up and using all the variables it declares.

func (sb *StmtBuilder) Stmt() ast.Stmt {
	// If the maximum allowed stmt depth has been reached, generate an
	// assign statement, since they don't nest.
	if sb.depth >= sb.conf.MaxStmtDepth {
		return sb.AssignStmt()
	}
	// sb.depth < sb.conf.MaxStmtDepth

	var s ast.Stmt
	switch n := sb.rs.Intn(7); n {
	case 0:
		// Generate a BranchStmt (break/continue) instead of an
		// assignment, with chance 0.25, but only if inside a loop.
		if sb.inloop && sb.rs.Intn(4) == 0 {
			s = sb.BranchStmt()
		} else {
			s = sb.AssignStmt()
		}
	case 1:
		sb.depth++
		s = sb.BlockStmt()
		sb.depth--
	case 2:
		sb.depth++
		// If at least one array or string is in scope, generate a for
		// range loop with chance 0.5; otherwise generate a plain loop
		arr, ok := sb.scope.GetRandomRangeable(sb.rs)
		if ok && sb.rs.Intn(2) == 0 {
			s = sb.RangeStmt(arr)
		} else {
			s = sb.ForStmt()
		}
		sb.depth--
	case 3:
		sb.depth++
		s = sb.IfStmt()
		sb.depth--
	case 4:
		sb.depth++
		s = sb.SwitchStmt()
		sb.depth--
	case 5:
		s = sb.SendStmt()
	case 6:
		sb.depth++
		s = sb.SelectStmt()
		sb.depth--
	default:
		panic("Stmt: bad index")
	}

	return s
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
		// TODO(alb): give the parameters names, add then to the
		// current scope, so they can be used in the body.

		// Generate a Block for the body, but only if we are not at
		// max depth.
		rhs := []ast.Expr{sb.eb.Expr(v.Type)}
		fl := rhs[0].(*ast.FuncLit)
		if sb.depth < sb.conf.MaxStmtDepth {
			oldInloop := sb.inloop
			sb.inloop = false

			sb.depth++
			fl.Body = sb.BlockStmt()
			sb.depth--

			sb.inloop = oldInloop
		}

		// Generate a return line (unconditionally).
		retStmt := &ast.ReturnStmt{Results: []ast.Expr{}}
		for _, ret := range t.Ret {
			retStmt.Results = append(retStmt.Results, sb.eb.Expr(ret))
		}
		fl.Body.List = append(fl.Body.List, retStmt)

		return &ast.AssignStmt{
			Lhs: []ast.Expr{v.Name},
			Tok: token.ASSIGN,
			Rhs: rhs,
		}

	default:
		return &ast.AssignStmt{
			Lhs: []ast.Expr{v.Name},
			Tok: token.ASSIGN,
			Rhs: []ast.Expr{sb.eb.Expr(v.Type)},
		}
	}
}

// return an arrays of n random Types. That can include basic types,
// arrays, chans, and randomly generated struct types.
func (sb *StmtBuilder) RandomTypes(n int) []Type {
	types := make([]Type, 0, n)

	// TODO(alb): better distribution of types. .2 are struct and .2
	// are function but it's always only one random type of each. Why
	// can't we have multiple different FuncTypes?

	// .2 of the returned types will be structs
	for i := 0; i < (1 + n/5); i++ {
		types = append(types, RandStructType(sb.conf.SupportedTypes))
	}
	n -= (n/5 + 1)

	// .2 of the returned types will be functions
	for i := 0; i < (1 + n/5); i++ {
		types = append(types, RandFuncType(sb.conf.SupportedTypes))
	}
	n -= (n/5 + 1)

	st := sb.conf.SupportedTypes
	for i := 0; i < n; i++ {
		t := st[sb.rs.Intn(len(st))]
		switch n := sb.rs.Intn(6); n {
		case 0:
			t = ArrOf(t)
		case 1:
			t2 := st[sb.rs.Intn(len(st))]
			t = MapOf(t, t2)
		case 2:
			t = PointerOf(t)
		case 3:
			t = ChanOf(t)
		default: // 3 < n < 6
			// 2/6 of keeping t as a plain variable
		}
		types = append(types, t)
	}

	return types
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

	sb.stats.Block++

	bs := new(ast.BlockStmt)
	stmts := []ast.Stmt{}

	// A new block means opening a new scope.
	//
	// First, declare nVars variables that will be in scope in this
	// block (together with the outer scopes ones).
	var nVars int
	if sb.conf.MaxBlockVars <= 5 {
		nVars = sb.conf.MaxBlockVars
	} else {
		nVars = 5 + sb.rs.Intn(sb.conf.MaxBlockVars-5)
	}

	var types []Type
	if nVars >= 8 {
		types = sb.RandomTypes(8)
	} else {
		types = sb.RandomTypes(nVars)
	}

	rs := RandSplit(nVars, len(types))

	var newVars []*ast.Ident
	for i, t := range types {
		if rs[i] > 0 {
			newDecl, nv := sb.DeclStmt(rs[i], t)
			stmts = append(stmts, newDecl)
			newVars = append(newVars, nv...)
		}
	}

	// We always want at least one addressable type in scope, put an
	// int which is generally more useful.
	newDecl, nv := sb.DeclStmt(1, BasicType{"int"})
	stmts = append(stmts, newDecl)
	newVars = append(newVars, nv...)

	var nStmts int
	if sb.depth == sb.conf.MaxStmtDepth {
		// at maxdepth, only assignments and incdec are allowed.
		// Guarantee 8 of them, to be sure not to generate
		// almost-empty blocks.
		nStmts = 8
	} else {
		// othewise, generate at least min(4, MaxBlockStmt) and at
		// most MaxBlockStmts.
		if sb.conf.MaxBlockStmts < 4 {
			nStmts = sb.conf.MaxBlockStmts
		} else {
			nStmts = 4 + sb.rs.Intn(sb.conf.MaxBlockStmts-3)
		}
	}

	// Fill the block's body with statements (but *no* new declaration:
	// we only use the variables we just declared, plus the ones in
	// scope when we enter the block).
	for i := 0; i < nStmts; i++ {
		stmts = append(stmts, sb.Stmt())
	}

	// Use all the newly declared variables...
	if len(newVars) > 0 {
		stmts = append(stmts, sb.UseVars(newVars))
	}

	// ... and then remove them from scope.
	for _, v := range newVars {
		sb.scope.DeleteIdentByName(v)
	}

	bs.List = stmts
	return bs
}

// DeclStmt returns a DeclStmt where nVars new variables of type kind
// are declared, and a list of the newly created *ast.Ident that
// entered the scope.
func (sb *StmtBuilder) DeclStmt(nVars int, t Type) (*ast.DeclStmt, []*ast.Ident) {
	if nVars < 1 {
		panic("DeclStmt: nVars < 1")
	}

	gd := new(ast.GenDecl)
	gd.Tok = token.VAR

	// generate nVars ast.Idents
	idents := make([]*ast.Ident, 0, nVars)
	for i := 0; i < nVars; i++ {
		idents = append(idents, sb.scope.NewIdent(t))
	}

	// generate the type specifier
	var typ ast.Expr

	switch t2 := t.(type) {
	case BasicType:
		typ = TypeIdent(t2.Name())
	case ArrayType:
		typ = &ast.ArrayType{Elt: TypeIdent(t2.Base().Name())}
	case PointerType:
		typ = &ast.StarExpr{X: TypeIdent(t2.Base().Name())}
	case StructType:
		typ = BuildStructAst(t2)
	case ChanType:
		typ = &ast.ChanType{Dir: 3, Value: TypeIdent(t2.Base().Name())}
	case MapType:
		typ = &ast.MapType{
			Key:   TypeIdent(t2.KeyT.Name()),
			Value: TypeIdent(t2.ValueT.Name()),
		}
	case FuncType:
		p, r := sb.eb.MakeFieldLists(t)
		typ = &ast.FuncType{Params: p, Results: r}
	default:
		panic("DeclStmt: bad type " + t.Name())
	}

	gd.Specs = []ast.Spec{
		&ast.ValueSpec{
			Names: idents,
			Type:  typ.(ast.Expr),
		},
	}

	ds := new(ast.DeclStmt)
	ds.Decl = gd

	return ds, idents
}

func BuildStructAst(t StructType) *ast.StructType {

	fields := make([]*ast.Field, 0, len(t.Fnames))

	for i := range t.Fnames {
		field := &ast.Field{
			Names: []*ast.Ident{&ast.Ident{Name: t.Fnames[i]}},
			Type:  TypeIdent(t.Ftypes[i].Name()),
		}
		fields = append(fields, field)
	}

	return &ast.StructType{
		Fields: &ast.FieldList{
			List: fields,
		},
	}
}

func (sb *StmtBuilder) ForStmt() *ast.ForStmt {
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

	sb.stats.For++
	sb.inloop = true
	defer func() { sb.inloop = false }()

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

	sb.stats.If++

	is := &ast.IfStmt{
		Cond: sb.eb.Expr(BasicType{"bool"}),
		Body: sb.BlockStmt(),
	}

	// optionally attach an 'else'
	if sb.rs.Float64() < 0.5 {
		is.Else = sb.BlockStmt()
	}

	return is
}

func (sb *StmtBuilder) SwitchStmt() *ast.SwitchStmt {

	sb.stats.Switch++

	t := RandType(sb.conf.SupportedTypes)
	if sb.rs.Int63()%2 == 0 && sb.scope.HasType(PointerOf(t)) {
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
	stmtList := []ast.Stmt{}
	for i := 0; i < 1+sb.rs.Intn(sb.conf.MaxBlockStmts); i++ {
		stmtList = append(stmtList, sb.Stmt())
	}

	cc := new(ast.CaseClause)
	if !def {
		cc.List = []ast.Expr{sb.eb.Expr(t)}
	}
	cc.Body = stmtList

	return cc
}

func (sb *StmtBuilder) CanIncDec() (Type, bool) {

	// disabled for now, IncDecStmt is too annoying to make it
	// work. The main issue is that the IncDecStmt is the only one we
	// can't generate unconditionally, since it requires an int or
	// float variable to be in scope.
	//
	// TODO(alb): re-enable
	return nil, false

}

// currently disabled
func (sb *StmtBuilder) IncDecStmt(t Type) *ast.IncDecStmt {
	return nil
}

func (sb *StmtBuilder) SendStmt() *ast.SendStmt {

	sb.stats.Send++

	// get the variable .Name
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
	sb.stats.Select++
	st := &ast.SelectStmt{
		Body: &ast.BlockStmt{List: []ast.Stmt{
			sb.CommClause(false),
			sb.CommClause(false),
			sb.CommClause(true),
		}},
	}

	return st
}

// CommClause is the Select clause. This function returns:
//   case <- [channel]     if def is false
//   default               if def is true
func (sb *StmtBuilder) CommClause(def bool) *ast.CommClause {

	// a couple of Stmt are enough for a select case body
	stmtList := []ast.Stmt{
		sb.Stmt(),
		sb.Stmt(),
	}

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
	panic("Should not be called")
	return nil
}

// ---------------- //
//       misc       //
// ---------------- //

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
