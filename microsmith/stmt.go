package microsmith

import (
	"go/ast"
	"go/token"
	"math/rand"
	"strings"
)

type StmtBuilder struct {
	rs    *rand.Rand // randomness source
	eb    *ExprBuilder
	depth int // how deep the stmt hyerarchy is
	conf  ProgramConf
	scope *Scope
}

type StmtConf struct {
	// maximum allowed blocks depth
	MaxStmtDepth int

	// chances of generating each type of Stmt. In order
	//   0. Assign Stmt
	//   1. Block Stmt
	//   2. For Stmt
	//   3. If Stmt
	//   4. Switch Stmt
	//   5. IncDec Stmt
	//   6. Send Stmt
	StmtKindChance []float64

	// max amount of variables and statements inside new block
	MaxBlockVars  int
	MaxBlockStmts int

	// whether to declare and use array variables
	UseArrays bool

	// whether to declare and use structs
	UseStructs bool

	// whether to declare and use channels
	UseChans bool

	// whether to declare and use pointer types
	UsePointers bool
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

	// a few functions from the math package
	scope = append(scope, Variable{
		MathSqrt,
		&ast.Ident{Name: MathSqrt.Name()}})
	scope = append(scope, Variable{
		MathMax,
		&ast.Ident{Name: MathMax.Name()}})
	scope = append(scope, Variable{
		MathMod,
		&ast.Ident{Name: MathMod.Name()}})

	scope = append(scope, Variable{
		RandIntn,
		&ast.Ident{Name: RandIntn.Name()}})

	// int() conversion are not enabled because int(Expr()) will fail
	// at compile-time if Expr() is a float64 expression made up of
	// only literals.
	//
	// scope = append(scope,
	// 	Variable{
	// 		IntConv,
	// 		&ast.Ident{Name: IntConv.Name()},
	// 	})

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
	// If the maximum allowed stmt depth has been reached, only
	// generate stmt that do not nest (assignments).
	if sb.depth >= sb.conf.MaxStmtDepth {
		return sb.AssignStmt()
	}
	// sb.depth < sb.conf.MaxStmtDepth

	switch RandIndex(sb.conf.StmtKindChance, sb.rs.Float64()) {
	case 0:
		return sb.AssignStmt()
	case 1:
		sb.depth++
		s := sb.BlockStmt()
		sb.depth--
		return s
	case 2:
		sb.depth++
		s := sb.ForStmt()
		sb.depth--
		return s
	case 3:
		sb.depth++
		s := sb.IfStmt()
		sb.depth--
		return s
	case 4:
		sb.depth++
		s := sb.SwitchStmt()
		sb.depth--
		return s
	case 5:
		if t, ok := sb.CanIncDec(); ok {
			s := sb.IncDecStmt(t)
			return s
		} else {
			// cannot incdec, fallback to an assignment
			s := sb.AssignStmt()
			return s
		}
	case 6:
		return sb.SendStmt()
		// return sb.ExprStmt()
	default:
		panic("Stmt: bad RandIndex")
	}
}

// gets a random variable currently in scope (that we can assign to),
// and builds an AssignStmt with a random Expr of its type on the RHS
func (sb *StmtBuilder) AssignStmt() *ast.AssignStmt {
	v := sb.scope.RandomVar(true)
	switch t := v.Type.(type) {
	case StructType:
		// we don't build struct literals yet, so if we got a struct
		// we'll assign to one of its fields.
		//
		// TODO(alb): implement support for struct literals in Expr,
		// then enable assignments to structs
		i := sb.rs.Intn(len(t.Fnames))
		return &ast.AssignStmt{
			// struct.field = <expr>
			Lhs: []ast.Expr{
				&ast.SelectorExpr{
					X:   v.Name,
					Sel: &ast.Ident{Name: t.Fnames[i]},
				}},
			Tok: token.ASSIGN,
			Rhs: []ast.Expr{sb.eb.Expr(t.Ftypes[i])},
		}
	case ArrayType:
		if !sb.conf.UseArrays {
			panic("AssignStmt: got array var, but arrays not enabled")
		}
		// if we got an array, 50/50 between
		//   1. AI = []int{ <exprs> }
		//   2. AI[<expr>] = <expr>
		if sb.rs.Intn(2) == 0 {
			return &ast.AssignStmt{
				Lhs: []ast.Expr{sb.eb.IndexExpr(t)},
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

	// we always want at least one addressable type
	types = append(types, RandType(
		[]Type{BasicType{"int"}, BasicType{"float64"},
			BasicType{"bool"}, BasicType{"string"}}))
	n--

	// if struct are enabled, .20 of the returned types will be
	// structs
	if sb.conf.UseStructs {
		for i := 0; i < (1 + n/5); i++ {
			types = append(types, RandStructType(sb.conf.SupportedTypes))
		}
		n -= (n/5 + 1)
	}

	st := sb.conf.SupportedTypes
	for i := 0; i < n; i++ {
		t := st[sb.rs.Intn(len(st))]

		if sb.conf.UseArrays && sb.rs.Intn(2) == 0 {
			t = ArrOf(t)
			types = append(types, t)
			continue
		}

		if sb.conf.UsePointers && sb.rs.Intn(2) == 0 {
			t = PointerOf(t)
			types = append(types, t)
			continue
		}

		if sb.conf.UseChans && sb.rs.Intn(4) == 0 {
			t = ChanOf(t)
			types = append(types, t)
			continue
		}
	}

	return types
}

// BlockStmt returns a new Block Statement. The returned Stmt is
// always a valid block. It up to BlockStmt's caller to make sure
// BlockStmt is only called when we have not yet reached max depth.
//
// TODO: move depth increment and decrement here(?)
func (sb *StmtBuilder) BlockStmt() *ast.BlockStmt {

	bs := new(ast.BlockStmt)
	stmts := []ast.Stmt{}

	// A new block means opening a new scope.
	//
	// First, declare nVars variables that will be in scope in this
	// block (together with the outer scopes ones).
	var nVars int
	if sb.conf.MaxBlockVars <= 4 {
		nVars = sb.conf.MaxBlockVars
	} else {
		nVars = 4 + sb.rs.Intn(sb.conf.MaxBlockVars-4)
	}

	var types []Type
	if nVars >= 5 {
		types = sb.RandomTypes(5)
	} else {
		types = sb.RandomTypes(nVars)
	}

	var newVars []*ast.Ident
	rs := RandSplit(nVars, len(types))

	for i, t := range types {
		if rs[i] > 0 {
			newDecl, nv := sb.DeclStmt(rs[i], t)
			stmts = append(stmts, newDecl)
			newVars = append(newVars, nv...)
		}
	}

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

	switch t := t.(type) {
	case BasicType:
		typ = TypeIdent(t.Name())
	case ArrayType:
		typ = &ast.ArrayType{Elt: TypeIdent(t.Base().Name())}
	case PointerType:
		typ = &ast.StarExpr{X: TypeIdent(t.Base().Name())}
	case StructType:
		typ = BuildStructAst(t)
	case ChanType:
		typ = &ast.ChanType{Dir: 3, Value: TypeIdent(t.Base().Name())}
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
	var fs ast.ForStmt

	// Add
	//   - a Cond stmt with chance 0.94 (1-1/16)
	//   - Init and Post statements with chance 0.5
	//   - A body with chance 0.97 (1-1/32)
	if sb.rs.Int63()%16 != 0 {
		fs.Cond = sb.eb.Expr(BasicType{"bool"})
	}
	if sb.rs.Int63()%2 == 0 {
		fs.Init = sb.AssignStmt()
	}
	if sb.rs.Int63()%2 == 0 {
		if t, ok := sb.CanIncDec(); ok && sb.rs.Int63()%2 == 0 {
			fs.Post = sb.IncDecStmt(t)
		} else {
			fs.Post = sb.AssignStmt()
		}
	}
	if sb.rs.Int63()%32 != 0 {
		fs.Body = sb.BlockStmt()
	} else {
		// empty loop body, still needs a BlockStmt
		fs.Body = &ast.BlockStmt{}
	}

	return &fs
}

func (sb *StmtBuilder) IfStmt() *ast.IfStmt {
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
	t := RandType(sb.conf.SupportedTypes)
	if sb.rs.Int63()%2 == 0 && sb.scope.TypeInScope(PointerOf(t)) {
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

// returns <nil, false> if there's no variable in scope we can inc or
// dec. Otherwise, returns <t, true>; where t is a random type that
// can be inc/dec and has a variable in scope.
func (sb *StmtBuilder) CanIncDec() (Type, bool) {

	// disabled for now, IncDecStmt is too annoying to make it
	// work. The main issue is that the IncDecStmt is the only one we
	// can't generate unconditionally, since it requires an int or
	// float variable to be in scope.
	//
	// TODO(alb): re-enable
	return nil, false

	// collect in scope types as int to avoid contT2I allocating every
	// time we put a Type in an interface Type array.
	inScope := make([]int, 0, 4)
	if sb.scope.TypeInScope(BasicType{"int"}) {
		inScope = append(inScope, 1)
	}
	if sb.scope.TypeInScope(ArrOf(BasicType{"int"})) {
		inScope = append(inScope, 2)
	}
	if sb.scope.TypeInScope(BasicType{"float64"}) {
		inScope = append(inScope, 3)
	}
	if sb.scope.TypeInScope(ArrOf(BasicType{"float64"})) {
		inScope = append(inScope, 4)
	}

	if len(inScope) == 0 {
		return nil, false
	}

	switch inScope[sb.rs.Int63n(int64(len(inScope)))] {
	case 1:
		return BasicType{"int"}, true
	case 2:
		return ArrOf(BasicType{"int"}), true
	case 3:
		return BasicType{"float64"}, true
	case 4:
		return ArrOf(BasicType{"float64"}), true
	default:
		panic("CanIncDec: bad type")
	}
}

func (sb *StmtBuilder) IncDecStmt(t Type) *ast.IncDecStmt {
	is := new(ast.IncDecStmt)
	if t.Name() == "int" || t.Name() == "float64" {
		is.X = sb.scope.RandomIdentExprAddressable(t, sb.rs)
	} else {
		is.X = sb.eb.IndexExpr(t)
	}

	if sb.rs.Int63()%2 == 0 {
		is.Tok = token.INC
	} else {
		is.Tok = token.DEC
	}

	return is
}

func (sb *StmtBuilder) SendStmt() *ast.SendStmt {

	// get the variable .Name
	st := new(ast.SendStmt)

	chs := sb.scope.InScopeChans()
	if len(chs) == 0 {
		// no channels in scope, but we can send to a brand
		// new one (e.g. make(chan int) <- 1)
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
		randChan := chs[sb.rs.Int63n(int64(len(chs)))]
		st.Chan = randChan.Name
		st.Value = sb.eb.Expr(randChan.Type.(ChanType).Base())
	}

	return st
}

// What is allowed, according to the spec:
//   - function and method calls
//   - receive operation
//
// Currently disabled because the code is a mess and it doesn't add
// much to the generated programs anyway.
func (sb *StmtBuilder) ExprStmt() *ast.ExprStmt {

	panic("Should not be called")

	es := new(ast.ExprStmt)

	for _, t := range sb.conf.SupportedTypes { // TODO(alb): add randomization(?)
		if funcs := sb.scope.InScopeFuncsReal(t); len(funcs) > 0 {
			// choose one of them at random
			fun := funcs[sb.rs.Intn(len(funcs))]
			name := fun.Name.Name
			if strings.HasPrefix(name, "math.") {
				es.X = sb.eb.MakeMathCall(fun)
			} else if strings.HasPrefix(name, "rand.") {
				es.X = sb.eb.MakeRandCall(fun)
			}
			return es
		}
	}

	// channel receive
	chs := sb.scope.InScopeChans()
	if len(chs) == 0 {
		// no channels in scope, we'll just make a new one
		t := RandType(sb.conf.SupportedTypes)
		es.X = &ast.UnaryExpr{
			Op: token.ARROW,
			X: &ast.CallExpr{
				Fun: &ast.Ident{Name: "make"},
				Args: []ast.Expr{
					&ast.ChanType{Dir: 3, Value: TypeIdent(t.Name())},
				},
			},
		}
	} else {
		randChan := chs[sb.rs.Int63n(int64(len(chs)))]
		es.X = &ast.UnaryExpr{
			Op: token.ARROW,
			X:  randChan.Name,
		}
	}

	return es
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
