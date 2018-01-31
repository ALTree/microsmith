package microsmith

import (
	"fmt"
	"go/ast"
	"go/token"
	"math/rand"
)

// Scope
type Scope []*ast.Ident

type StmtBuilder struct {
	rs    *rand.Rand // randomness source
	eb    *ExprBuilder
	depth int // how deep the stmt hyerarchy is
	conf  StmtConf

	// map from type to Scope set. For example
	//   map[int]
	// points to a scope type (define above) holding all int
	// variables that are in scope.
	// Q: why is this here?
	// A: because it's StmtBuilder that create new scopes (for now)
	inScope map[Type]Scope
}

type StmtConf struct {
	// maximum allowed blocks depth
	maxStmtDepth int

	// chances of generating each type of Stmt. In order
	//   0. Assign Stmt
	//   1. Block Stmt
	//   2. For Stmt
	//   3. If Stmt
	//   4. Switch Stmt
	stmtKindChance []float64

	// max amount of variables and statements inside new block
	maxBlockVars  int
	maxBlockStmts int
}

func NewStmtBuilder(rs *rand.Rand) *StmtBuilder {
	sb := new(StmtBuilder)
	sb.rs = rs
	sb.conf = StmtConf{
		maxStmtDepth: 2,
		stmtKindChance: []float64{
			4, 2, 2, 2, 1,
		},
		maxBlockVars:  2 * (3 * len(SupportedTypes)),
		maxBlockStmts: 10,
	}

	// initialize scope structures
	scpMap := make(map[Type]Scope)
	for _, t := range SupportedTypes {
		scpMap[t] = Scope{}
		scpMap[t.Arr()] = Scope{}
	}

	sb.inScope = scpMap
	sb.eb = NewExprBuilder(rs, sb.inScope)

	return sb
}

// AddIdent adds a new variable of 'kind' type to the global scope, and
// returns a pointer to it.
func (sb *StmtBuilder) AddIdent(t Type) *ast.Ident {

	inScope := sb.inScope[t]

	name := fmt.Sprintf("%s%v", t.VarName(), len(inScope))

	// build Ident object
	id := new(ast.Ident)
	id.Obj = &ast.Object{Kind: ast.Var, Name: name}
	id.Name = name

	// add to kind scope
	inScope = append(inScope, id)
	sb.inScope[t] = inScope

	return id
}

// DeleteIdent deletes the id-th Ident of type kind from its scope.
// If id < 0, it deletes the last one that was declared.
func (sb *StmtBuilder) DeleteIdent(t Type, id int) {
	inScope := sb.inScope[t]
	if id < 0 {
		inScope = inScope[:len(inScope)-1]
	} else {
		inScope = append(inScope[:id], inScope[id+1:]...)
	}
	sb.inScope[t] = inScope
}

// RandomInScopeVar returns a random variable from the given Scope.
// Panics if the scope is empty.
func RandomInScopeVar(inScope Scope, rs *rand.Rand) *ast.Ident {
	if len(inScope) == 0 {
		panic("RandomInScopeVar: empty scope")
	}
	return inScope[rs.Intn(len(inScope))]
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
//
// DeclStmt is implemented and used, but is not used directly here (by
// Stmt. It's only used inside BlockStmt, which takes care of setting
// up and using all the variables it declares.

func (sb *StmtBuilder) Stmt() ast.Stmt {

	// types that have at least one variable currently in scope
	inScopeKinds := []Type{}
	for _, st := range SupportedTypes {
		if len(sb.inScope[st]) > 0 {
			inScopeKinds = append(inScopeKinds, st)
		}
	}

	if len(inScopeKinds) == 0 {
		panic("Stmt: no inscope variables")
	}

	if sb.depth >= sb.conf.maxStmtDepth {
		return sb.AssignStmt(RandType(inScopeKinds))
	}
	// sb.depth < sb.conf.maxStmtDepth

	switch RandIndex(sb.conf.stmtKindChance, sb.rs.Float64()) {
	case 0:
		return sb.AssignStmt(RandType(inScopeKinds))
	case 1:
		sb.depth++
		s := sb.BlockStmt(0, 0)
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
	default:
		panic("Stmt: bad RandIndex")
	}
}

// Build an assign statement with a random inscope variables of type
// kind. panics if there isn't one in scope.
func (sb *StmtBuilder) AssignStmt(t Type) *ast.AssignStmt {
	v := RandomInScopeVar(sb.inScope[t], sb.rs)
	if v == nil {
		panic("AssignStmt: empty scope")
	}

	as := new(ast.AssignStmt)
	as.Lhs = []ast.Expr{v}
	as.Tok = token.ASSIGN
	as.Rhs = []ast.Expr{sb.eb.Expr(t)}

	return as
}

// BlockStmt returns a new Block Statement. The returned Stmt is
// always a valid block. It up to BlockStmt's caller to make sure
// BlockStmt is only called when we have not yet reached max depth.
//
// nVars controls the number of new variables declared at the top of
// the block. nStmt controls the number of new statements that will be
// included in the returned block. If there are zero, BlockStmt will
// decide by itself how many new variables it'll declare or how many
// statement to include in the block.
//
// TODO: move depth increment and decrement here(?)
// TODO: make nVars the total, not nVars of each type
func (sb *StmtBuilder) BlockStmt(nVars, nStmts int) *ast.BlockStmt {

	bs := new(ast.BlockStmt)
	stmts := []ast.Stmt{}

	// A new block means opening a new scope.
	//
	// First, declare nVars variables that will be in scope in this
	// block (together with the outer scopes ones).
	if nVars < 1 {
		nVars = sb.conf.maxBlockVars
	}

	// SupportedTypes := []Type{
	// 	TypeInt, TypeBool, TypeString,
	// 	TypeIntArr, TypeBoolArr, TypeStringArr,
	// }

	// nVarsByKind[kind] holds a random nonnegative integer that
	// indicates how many new variables of type kind we'll declare in
	// a minute.
	nVarsByKind := make(map[Type]int)
	rs := RandSplit(nVars, 2*len(SupportedTypes)) // 2* because of the Array Types
	for i, v := range SupportedTypes {
		nVarsByKind[v] = rs[i]
		nVarsByKind[v.Arr()] = rs[2*i+1]
	}

	// Declare the new variables mentioned above.  Save their names in
	// newVars so that later we can generate a statement that uses
	// them before exiting the block scope (to avoid 'unused' errors).
	var newVars []*ast.Ident
	for typ, nVar := range nVarsByKind {
		if nVar > 0 {
			newDecl, nv := sb.DeclStmt(nVar, typ)
			stmts = append(stmts, newDecl)
			newVars = append(newVars, nv...)
		}
	}

	// Fill the block's body with statements (but *no* new
	// declaration: we only use the variables we just declared, plus
	// the ones in scope when we enter the block).
	if nStmts < 1 {
		// we want at least 5 statements
		nStmts = 5 + sb.rs.Intn(sb.conf.maxBlockStmts-5)
	}
	for i := 0; i < nStmts; i++ {
		stmts = append(stmts, sb.Stmt())
	}

	// Use all the newly declared variables...
	if len(newVars) > 0 {
		stmts = append(stmts, sb.UseVars(newVars))
	}
	// ... and then remove them from scope.
	for typ, nVar := range nVarsByKind {
		for i := 0; i < nVar; i++ {
			sb.DeleteIdent(typ, -1)
		}
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
	idents := make([]*ast.Ident, 0)
	for i := 0; i < nVars; i++ {
		idents = append(idents, sb.AddIdent(t))
	}

	// generate the type specifier
	var typ ast.Expr
	if t.IsBasic() {
		typ = &ast.Ident{Name: t.String()}
	} else {
		typ = &ast.ArrayType{Elt: &ast.Ident{Name: t.Base().String()}}
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

func (sb *StmtBuilder) ForStmt() *ast.ForStmt {
	var fs *ast.ForStmt
	if sb.rs.Float64() < 1 {
		fs = &ast.ForStmt{
			Cond: sb.eb.Expr(TypeBool),
			Body: sb.BlockStmt(0, 0),
		}
	} else {
		// TODO: for init; cond; post { ..
	}

	return fs
}

func (sb *StmtBuilder) IfStmt() *ast.IfStmt {
	is := &ast.IfStmt{
		Cond: sb.eb.Expr(TypeBool),
		Body: sb.BlockStmt(0, 0),
	}

	// optionally attach an 'else'
	if sb.rs.Float64() < 0.5 {
		is.Else = sb.BlockStmt(0, 0)
	}

	return is
}

func (sb *StmtBuilder) SwitchStmt() *ast.SwitchStmt {
	t := RandType(SupportedTypes)
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
	for i := 0; i < 1+sb.rs.Intn(sb.conf.maxBlockStmts/2); i++ {
		stmtList = append(stmtList, sb.Stmt())
	}

	cc := new(ast.CaseClause)
	if !def {
		cc.List = []ast.Expr{sb.eb.Expr(t)}
	}
	cc.Body = stmtList

	return cc
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

// build and return a statement of form
//   _, _, ... _ = var1, var2, ..., varN
// for each ident in idents
func (sb *StmtBuilder) UseVars(idents []*ast.Ident) ast.Stmt {
	useStmt := &ast.AssignStmt{Tok: token.ASSIGN}
	for _, name := range idents {
		useStmt.Lhs = append(useStmt.Lhs, &ast.Ident{Name: "_"})
		useStmt.Rhs = append(useStmt.Rhs, name)
	}
	return useStmt
}
