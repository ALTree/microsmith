package microsmith

import (
	"fmt"
	"go/ast"
	"go/token"
	"math/rand"
	"strings"
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
	inScope map[string]Scope
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
			3, 1, 1, 1, 1,
		},
		maxBlockVars:  3,
		maxBlockStmts: 5,
	}

	// initialize scope structures
	scpMap := make(map[string]Scope)
	for _, t := range []string{"int", "bool"} {
		scpMap[t] = Scope{}
	}
	sb.inScope = scpMap

	sb.eb = NewExprBuilder(rs, sb.inScope)

	return sb
}

// AddIdent adds a new variable of 'kind' type to the global scope, and
// returns a pointer to it.
func (sb *StmtBuilder) AddIdent(kind string) *ast.Ident {

	inScope := sb.inScope[kind]

	name := fmt.Sprintf("%s%v", strings.Title(kind)[:1], len(inScope))

	// build Ident object
	id := new(ast.Ident)
	id.Obj = &ast.Object{Kind: ast.Var, Name: name}
	id.Name = name

	// add to kind scope
	inScope = append(inScope, id)
	sb.inScope[kind] = inScope

	return id
}

// DeleteIdent deletes the id-th Ident of type kind from its scope.
// If id < 0, it deletes the last one that was declared.
func (sb *StmtBuilder) DeleteIdent(kind string, id int) {
	inScope := sb.inScope[kind]
	if id < 0 {
		inScope = inScope[:len(inScope)-1]
	} else {
		inScope = append(inScope[:id], inScope[id+1:]...)
	}
	sb.inScope[kind] = inScope
}

// RandomInScopeVar returns a random variable from the given Scope
func RandomInScopeVar(inScope Scope, rs *rand.Rand) *ast.Ident {
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
	switch RandIndex(sb.conf.stmtKindChance, sb.rs.Float64()) {
	case 0:
		ttt := sb.rs.Uint32() % 2 // TODO: generalize on types
		if ttt == 0 {
			return sb.AssignStmt("int")
		}
		return sb.AssignStmt("bool")
	case 1:
		if sb.depth >= sb.conf.maxStmtDepth {
			return &ast.EmptyStmt{}
		}
		sb.depth++
		s := sb.BlockStmt(0, 0)
		sb.depth--
		return s
	case 2:
		if sb.depth >= sb.conf.maxStmtDepth {
			return &ast.EmptyStmt{}
		}
		sb.depth++
		s := sb.ForStmt()
		sb.depth--
		return s
	case 3:
		if sb.depth >= sb.conf.maxStmtDepth {
			return &ast.EmptyStmt{}
		}
		sb.depth++ // If's body creates a block
		s := sb.IfStmt()
		sb.depth--
		return s
	case 4:
		if sb.depth >= sb.conf.maxStmtDepth {
			return &ast.EmptyStmt{}
		}
		sb.depth++
		s := sb.SwitchStmt()
		sb.depth--
		return s
	default:
		panic("Stmt: bad RandIndex")
	}
}

func (sb *StmtBuilder) AssignStmt(kind string) *ast.AssignStmt {
	as := new(ast.AssignStmt)

	as.Lhs = []ast.Expr{RandomInScopeVar(sb.inScope[kind], sb.rs)}
	as.Tok = token.ASSIGN
	as.Rhs = []ast.Expr{sb.eb.Expr(kind)}

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
	//
	// TODO: make the number of new variables inversely proportional
	// to the current depth (the deeper we are, the more variables
	// declared outside the block and already in scope we have).
	// If we do this, remember to update BlockStmt callers to pass
	// nVars < 1 so that BlockStmt will choose nVars by itself.
	if nVars < 1 {
		nVars = 2 + sb.rs.Intn(sb.conf.maxBlockVars-1)
	}
	newVarInts := sb.DeclStmt(nVars, "int")
	stmts = append(stmts, newVarInts)
	newVarBools := sb.DeclStmt(nVars, "bool")
	stmts = append(stmts, newVarBools)

	// Fill the block's body with statements (but *no* new
	// declaration: we only use the variables we just declared, plus
	// the ones in scope when we enter the block).
	if nStmts < 1 {
		nStmts = 2 + sb.rs.Intn(sb.conf.maxBlockStmts-1)
	}
	for i := 0; i < nStmts; i++ {
		stmts = append(stmts, sb.Stmt())
	}

	// Now we need to cleanup the new declared variables.
	newIntIdents := newVarInts.Decl.(*ast.GenDecl).Specs[0].(*ast.ValueSpec).Names
	newBoolIdents := newVarBools.Decl.(*ast.GenDecl).Specs[0].(*ast.ValueSpec).Names

	// First, use all of them to avoid 'unused' errors:
	stmts = append(stmts, sb.UseVars(newIntIdents))
	stmts = append(stmts, sb.UseVars(newBoolIdents))
	bs.List = stmts

	// And now remove then from inScope, since they'll no longer be in
	// scope when we leave this block.
	for i := 0; i < nVars; i++ {
		sb.DeleteIdent("int", -1)
		sb.DeleteIdent("bool", -1)
	}

	return bs
}

// DeclStmt returns a DeclStmt where nVars new variables of type kind
// are declared.
func (sb *StmtBuilder) DeclStmt(nVars int, kind string) *ast.DeclStmt {
	gd := new(ast.GenDecl)
	gd.Tok = token.VAR

	// generate nVars ast.Idents
	idents := make([]*ast.Ident, 0)
	for i := 0; i < nVars; i++ {
		idents = append(idents, sb.AddIdent(kind))
	}

	gd.Specs = []ast.Spec{
		&ast.ValueSpec{
			Names: idents,
			Type:  &ast.Ident{Name: kind},
		},
	}

	ds := new(ast.DeclStmt)
	ds.Decl = gd
	return ds
}

func (sb *StmtBuilder) ForStmt() *ast.ForStmt {
	var fs *ast.ForStmt
	if sb.rs.Float64() < 1 {
		fs = &ast.ForStmt{
			Cond: sb.eb.Expr("bool"),
			Body: sb.BlockStmt(0, 0),
		}
	} else {
		// TODO: for init; cond; post { ..
	}

	return fs
}

func (sb *StmtBuilder) IfStmt() *ast.IfStmt {
	is := &ast.IfStmt{
		Cond: sb.eb.Expr("bool"),
		Body: sb.BlockStmt(0, 0),
	}

	// optionally attach an 'else'
	if sb.rs.Float64() < 0.5 {
		is.Else = sb.BlockStmt(0, 0)
	}

	return is
}

func (sb *StmtBuilder) SwitchStmt() *ast.SwitchStmt {
	var kind string
	if sb.rs.Uint32()%2 == 0 {
		kind = "int"
	} else {
		kind = "bool"
	}
	ss := &ast.SwitchStmt{
		Tag: sb.eb.Expr(kind),
		Body: &ast.BlockStmt{
			List: []ast.Stmt{sb.CaseClause(kind)},
		},
	}

	return ss
}

// builds and returns a single CaseClause switching on type kind
func (sb *StmtBuilder) CaseClause(kind string) *ast.CaseClause {
	// generate up to maxBlockStmts
	stmtList := []ast.Stmt{}
	for i := 0; i < 1+sb.rs.Intn(sb.conf.maxBlockStmts); i++ {
		stmtList = append(stmtList, sb.Stmt())
	}

	cc := new(ast.CaseClause)
	cc.List = []ast.Expr{sb.eb.Expr(kind)}
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
func (sb *StmtBuilder) ExprStmt(kind string) *ast.ExprStmt {
	es := new(ast.ExprStmt)
	es.X = sb.eb.Expr(kind)
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
