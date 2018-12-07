package microsmith

import (
	"go/ast"
	"go/token"
	"math/rand"
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
	StmtKindChance []float64

	// max amount of variables and statements inside new block
	MaxBlockVars  int
	MaxBlockStmts int

	// whether to declare and use array variables
	UseArrays bool

	// whether to declare and use structs
	UseStructs bool

	// whether to declare and use pointer types
	UsePointers bool
}

func NewStmtBuilder(rs *rand.Rand, conf ProgramConf) *StmtBuilder {
	sb := new(StmtBuilder)
	sb.rs = rs
	sb.conf = conf

	if sb.conf.UseArrays {
		sb.conf.MaxBlockVars *= 2
	}

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
//   - IndDecStmt
//
// DeclStmt is implemented and used, but is not used directly here (by
// Stmt. It's only used inside BlockStmt, which takes care of setting
// up and using all the variables it declares.

func (sb *StmtBuilder) Stmt() ast.Stmt {

	// types that have at least one variable currently in scope
	typesInScope := sb.scope.InScopeTypes()

	if len(typesInScope) == 0 {
		panic("Stmt: no inscope variables")
	}

	// If the maximum allowed stmt depth has been reached, only
	// generate stmt that do not nest (assignments and incdec).
	if sb.depth >= sb.conf.MaxStmtDepth {
		if t, ok := sb.CanIncDec(); ok && sb.rs.Int63()%4 == 0 {
			return sb.IncDecStmt(t)
		} else {
			return sb.AssignStmt(RandType(typesInScope))
		}
	}
	// sb.depth < sb.conf.MaxStmtDepth

	switch RandIndex(sb.conf.StmtKindChance, sb.rs.Float64()) {
	case 0:
		return sb.AssignStmt(RandType(typesInScope))
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
		if t, ok := sb.CanIncDec(); ok && sb.rs.Int63()%4 == 0 {
			s := sb.IncDecStmt(t)
			return s
		} else {
			s := sb.AssignStmt(RandType(typesInScope))
			return s
		}
	default:
		panic("Stmt: bad RandIndex")
	}
}

// Build an assign statement with a random variable of type t.
func (sb *StmtBuilder) AssignStmt(t Type) *ast.AssignStmt {
	var v ast.Expr
	switch t := t.(type) {
	case BasicType:
		if sb.conf.UseArrays &&
			sb.scope.TypeInScope(ArrOf(t)) &&
			sb.rs.Float64() < sb.conf.IndexChance {
			v = sb.eb.IndexExpr(ArrOf(t))
		} else {
			v = sb.scope.RandomIdentExpr(t, sb.rs)
		}
	default:
		v = sb.scope.RandomIdentExpr(t, sb.rs)
	}

	as := &ast.AssignStmt{
		Lhs: []ast.Expr{v},
		Tok: token.ASSIGN,
		Rhs: []ast.Expr{sb.eb.Expr(t)},
	}

	return as
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
	nVars := 1 + sb.rs.Intn(sb.conf.MaxBlockVars)

	var ns int = 0 // number of struct we'll declare

	// If structs are enabled, 20% of our declaration will be of
	// structs
	if sb.conf.UseStructs {
		ns = nVars / 5
		nVars -= ns
	}

	rs := RandSplit(nVars, len(sb.conf.SupportedTypes))

	// Declare the new variables mentioned above.  Save their names in
	// newVars so that later we can generate a statement that uses
	// them before exiting the block scope (to avoid 'unused' errors).
	var newVars []*ast.Ident
	for i, t := range sb.conf.SupportedTypes {
		if rs[i] > 0 {
			typ := t

			// respectively the number of plain, array, and pointer
			// variables of type t we'll declare
			n, na, np := rs[i], 0, 0

			// If arrays are enabled, 25% of our declaration will be of arrays
			if sb.conf.UseArrays {
				na = n / 4
			}

			// If pointers are enabled, 25% of our declaration will be
			// of pointer type
			if sb.conf.UseArrays {
				np = n / 4
			}

			n -= (na + np)

			if n > 0 {
				newDecl, nv := sb.DeclStmt(n, typ)
				stmts = append(stmts, newDecl)
				newVars = append(newVars, nv...)
			}

			if na > 0 {
				newDecl, nv := sb.DeclStmt(na, ArrOf(typ))
				stmts = append(stmts, newDecl)
				newVars = append(newVars, nv...)
			}

			if np > 0 {
				newDecl, nv := sb.DeclStmt(np, PointerOf(typ))
				stmts = append(stmts, newDecl)
				newVars = append(newVars, nv...)
			}
		}
	}

	for i := 0; i < ns; i++ {
		newDecl, nv := sb.DeclStmt(1, RandStructType(sb.conf.SupportedTypes))
		stmts = append(stmts, newDecl)
		newVars = append(newVars, nv...)
	}

	if len(newVars) != ns+nVars {
		panic("BlockStmt: variable count mismatch")
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
		fs.Init = sb.AssignStmt(RandType(sb.scope.InScopeTypes()))
	}
	if sb.rs.Int63()%2 == 0 {
		if t, ok := sb.CanIncDec(); ok && sb.rs.Int63()%2 == 0 {
			fs.Post = sb.IncDecStmt(t)
		} else {
			fs.Post = sb.AssignStmt(RandType(sb.scope.InScopeTypes()))
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
	inScope := make([]Type, 0)
	if sb.scope.TypeInScope(BasicType{"int"}) {
		inScope = append(inScope, BasicType{"int"})
	}
	if sb.scope.TypeInScope(ArrOf(BasicType{"int"})) {
		inScope = append(inScope, ArrOf(BasicType{"int"}))
	}
	if sb.scope.TypeInScope(BasicType{"float64"}) {
		inScope = append(inScope, BasicType{"float64"})
	}
	if sb.scope.TypeInScope(ArrOf(BasicType{"float64"})) {
		inScope = append(inScope, ArrOf(BasicType{"float64"}))
	}

	if len(inScope) == 0 {
		return nil, false
	}

	return RandType(inScope), true
}

func (sb *StmtBuilder) IncDecStmt(t Type) *ast.IncDecStmt {
	is := new(ast.IncDecStmt)
	if t.Name() == "int" || t.Name() == "float64" {
		is.X = sb.scope.RandomIdentExpr(t, sb.rs)
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

// Spec says this cannot be any Expr.
// Rigth now we generate things like
//   1+3
// which do not compile. What is allowed:
//   - function and method calls
//   - receive operation
// ex:
//   h(x+y)
//   f.Close()
//   <-ch
//   (<-ch)
// TODO: when we have chans and/or funcs, fix and enable this
func (sb *StmtBuilder) ExprStmt(t Type) *ast.ExprStmt {
	es := new(ast.ExprStmt)
	es.X = sb.eb.Expr(t)
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
