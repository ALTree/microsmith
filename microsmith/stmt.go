package microsmith

import (
	"fmt"
	"go/ast"
	"go/token"
	"math/rand"
	"strings"
)

type StmtBuilder struct {
	rs    *rand.Rand // randomness source
	eb    *ExprBuilder
	depth int // how deep the stmt hyerarchy is
	conf  StmtConf

	// map from type to Scope set. For example
	//   map[int]
	// points to a scope type (define below) holding all int variables
	// that are in scope.
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
	stmtKindChance []float64
}

func NewStmtBuilder(rs *rand.Rand) *StmtBuilder {
	sb := new(StmtBuilder)
	sb.rs = rs
	sb.conf = StmtConf{
		maxStmtDepth: 2,
		stmtKindChance: []float64{
			3, 1, 1, 1,
		},
	}

	// initialize scope structures
	scpMap := make(map[string]Scope)
	for _, t := range []string{"int", "bool"} {
		scpMap[t] = make(map[string]*ast.Ident)
	}
	sb.inScope = scpMap

	sb.eb = NewExprBuilder(rs, sb.inScope)

	return sb
}

// Scope is a set of in-scope variables having the same type.
// We use a map from the variable name to its ast.Ident so that we can
// return an in-scope identifier without allocating a new ast.Ident
// every time.
// TODO: is this necessary? Would a map[string]struct{} be enough?
type Scope map[string]*ast.Ident

// TODO: pre-generate names and then draw them(?)
func (sb *StmtBuilder) VarIdent(kind string) *ast.Ident {
	name := fmt.Sprintf("%s%v", strings.Title(kind)[:1], sb.rs.Intn(1000))

	// try to generate a var name until we hit one that is not already
	// in function scope
	inScope := sb.inScope[kind]
	for _, ok := inScope[name]; ok; _, ok = inScope[name] {
		name = fmt.Sprintf("%s%v", strings.Title(kind)[:1], sb.rs.Intn(1000))
	}

	// build Ident object and return
	id := new(ast.Ident)
	id.Obj = &ast.Object{Kind: ast.Var, Name: name}
	id.Name = name

	inScope[name] = id

	return id
}

// TODO: delete (see below)
func (sb *StmtBuilder) RandomInScopeVar(kind string) *ast.Ident {
	inScope := sb.inScope[kind]
	i := sb.rs.Intn(len(inScope))
	counter := 0
	for _, v := range inScope {
		if i == counter {
			return v
		}
		counter++
	}

	panic("unreachable")
}

// We have this because ExprBuilders too need to call this
// TODO: only keep this one
func RandomInScopeVar(inScope Scope, rs *rand.Rand) *ast.Ident {
	i := rs.Intn(len(inScope))
	counter := 0
	for _, v := range inScope {
		if i == counter {
			return v
		}
		counter++
	}

	panic("unreachable")
}

// ---------------- //
//       Stmt       //
// ---------------- //

// Currently we generate:
//   - AssignStmt
//   - BlockStmt
//   - ForStmt
//   - IfStmt
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
		s := sb.BlockStmt(2, 4)
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
	default:
		panic("Stmt: bad RandIndex")
	}
}

func (sb *StmtBuilder) AssignStmt(kind string) *ast.AssignStmt {
	as := new(ast.AssignStmt)

	as.Lhs = []ast.Expr{sb.RandomInScopeVar(kind)}
	as.Tok = token.ASSIGN
	as.Rhs = []ast.Expr{sb.eb.Expr(kind)}

	return as
}

// BlockStmt returns a new Block Statement. The returned Stmt is
// always a valid block. It up to BlockStmt's caller to make sure
// BlockStmt is only called when we have not yet reached max depth.

// nVars controls the number of new variables declared at the top of
// the block.
//
// nStmt controls the number of new statements that will be included
// in the returned block.
//
// TODO: move depth increment and decrement here(?)
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
		nVars = 4
	}

	newVarInts := sb.DeclStmt(nVars, "int")
	stmts = append(stmts, newVarInts)
	newVarBools := sb.DeclStmt(nVars, "bool")
	stmts = append(stmts, newVarBools)

	// now fill the block's body with statements (but *no* new
	// declaration: we only use the variables we just declared, plus
	// the ones in scope when we enter the block).
	for i := 0; i < nStmts; i++ {
		stmts = append(stmts, sb.Stmt())
	}

	// now we need to cleanup the new declared variables.
	newIntIdents := newVarInts.Decl.(*ast.GenDecl).Specs[0].(*ast.ValueSpec).Names
	newBoolIdents := newVarBools.Decl.(*ast.GenDecl).Specs[0].(*ast.ValueSpec).Names

	// First, use all of them to avoid 'unused' errors:
	stmts = append(stmts, sb.UseVars(newIntIdents))
	stmts = append(stmts, sb.UseVars(newBoolIdents))
	bs.List = stmts

	// Finally, remove from scope the variables we declared at the
	// beginning of this block
	for _, v := range newIntIdents {
		delete(sb.inScope["int"], v.Name)
	}

	for _, v := range newBoolIdents {
		delete(sb.inScope["bool"], v.Name)
	}

	return bs
}

func (sb *StmtBuilder) DeclStmt(nVars int, kind string) *ast.DeclStmt {
	gd := new(ast.GenDecl)
	gd.Tok = token.VAR

	// generate nVars ast.Idents
	idents := make([]*ast.Ident, 0)
	for i := 0; i < nVars; i++ {
		idents = append(idents, sb.VarIdent(kind))
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
	fs := &ast.ForStmt{
		Cond: sb.eb.Expr("bool"),
		Body: sb.BlockStmt(0, 2),
	}

	return fs
}

func (sb *StmtBuilder) IfStmt() *ast.IfStmt {
	is := &ast.IfStmt{
		Cond: sb.eb.Expr("bool"),
		Body: sb.BlockStmt(2, 4),
	}

	// optionally attach an 'else'
	if sb.rs.Float64() < 0.5 {
		is.Else = sb.BlockStmt(2, 4)
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
//
// h(x+y)
// f.Close()
// <-ch
// (<-ch)
// TODO: fix
func (sb *StmtBuilder) ExprStmt(kind string) *ast.ExprStmt {
	es := new(ast.ExprStmt)
	es.X = sb.eb.Expr(kind)
	return es
}

// ---------------- //
//       misc       //
// ---------------- //

// generate and return a statement in the form
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
