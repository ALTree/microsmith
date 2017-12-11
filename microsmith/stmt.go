package microsmith

import (
	"fmt"
	"go/ast"
	"go/token"
	"math/rand"
)

const MaxStmtDepth = 2

type StmtBuilder struct {
	rs    *rand.Rand // randomness source
	eb    *ExprBuilder
	depth int // how deep the stmt hyerarchy is

	// list of variables that are in scope
	// Q: why is this here?
	// A: because it's StmtBuilder that create new scopes (for now)
	inScope map[string]*ast.Ident
}

func NewStmtBuilder(rs *rand.Rand) *StmtBuilder {
	sb := new(StmtBuilder)
	sb.rs = rs
	sb.eb = NewExprBuilder(rs)
	sb.inScope = make(map[string]*ast.Ident)
	return sb
}

// TODO: pre-generate names and then draw them(?)
func (sb *StmtBuilder) VarIdent() *ast.Ident {

	// try to generate a var name until we hit one that is not already
	// in function scope
	inScope := sb.inScope

	name := fmt.Sprintf("Var%v", sb.rs.Intn(1000))
	for _, ok := inScope[name]; ok; _, ok = inScope[name] {
		name = fmt.Sprintf("Var%v", sb.rs.Intn(1000))
	}

	// build Ident object and return
	id := new(ast.Ident)
	id.Obj = &ast.Object{Kind: ast.Var, Name: name}
	id.Name = name

	inScope[name] = id
	sb.inScope = inScope

	return id
}

func (sb *StmtBuilder) Stmt() ast.Stmt {
	// Currently we generate
	//   - AssignStmt
	//   - BlockStmt
	nFuncs := uint32(2)

	switch sb.rs.Uint32() % nFuncs {
	case 0:
		return sb.AssignStmt()
	case 1:
		sb.depth++
		if sb.depth > MaxStmtDepth {
			return &ast.EmptyStmt{}
		}
		s := sb.BlockStmt(3)
		sb.depth--
		return s
	// case 2:
	// 	return sb.ExprStmt()
	default:
		panic("Stmt: bad random")
	}
}

func (sb *StmtBuilder) RandomInScopeVar() *ast.Ident {
	nVars := len(sb.inScope)
	i := sb.rs.Intn(nVars)
	counter := 0
	for _, v := range sb.inScope {
		if i == counter {
			return v
		}
		counter++
	}

	panic("unreachable")
}

func (sb *StmtBuilder) AssignStmt() *ast.AssignStmt {
	as := new(ast.AssignStmt)

	as.Lhs = []ast.Expr{sb.RandomInScopeVar()}
	as.Tok = token.ASSIGN
	as.Rhs = []ast.Expr{sb.eb.Expr()}

	return as
}

func (sb *StmtBuilder) BlockStmt(nVars int) *ast.BlockStmt {
	bs := new(ast.BlockStmt)
	stmts := []ast.Stmt{}

	// A new block means opening a new scope.
	//
	// First, declare nVars variables that will be in scope in this
	// block (together with the outer scopes ones).
	if nVars < 1 {
		nVars = 4
	}
	newVars := sb.DeclStmt(nVars, "int")
	stmts = append(stmts, newVars)

	// now build the block body (with *no* new declaration)
	for i := 0; i < 4; i++ {
		stmts = append(stmts, sb.Stmt())
	}

	// now we need to cleanup the new declared variables.
	newVarsIdents := newVars.Decl.(*ast.GenDecl).Specs[0].(*ast.ValueSpec).Names

	// First, use all of them to avoid 'unused' errors:
	stmts = append(stmts, sb.UseVars(newVarsIdents))
	bs.List = stmts

	// Finally, remove from scope the variables we declared at the
	// beginning of this block
	for _, v := range newVarsIdents {
		delete(sb.inScope, v.Name)
	}

	return bs
}

func (sb *StmtBuilder) DeclStmt(nVars int, kind string) *ast.DeclStmt {
	gd := new(ast.GenDecl)
	gd.Tok = token.VAR

	// generate nVars ast.Idents
	idents := make([]*ast.Ident, 0)
	for i := 0; i < nVars; i++ {
		idents = append(idents, sb.VarIdent())
	}

	gd.Specs = []ast.Spec{
		&ast.ValueSpec{
			Names: idents,
			Type:  &ast.Ident{Name: "int"},
		},
	}

	ds := new(ast.DeclStmt)
	ds.Decl = gd
	return ds
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
func (sb *StmtBuilder) ExprStmt() *ast.ExprStmt {
	es := new(ast.ExprStmt)
	es.X = sb.eb.Expr()
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
