package microsmith

import (
	"fmt"
	"go/ast"
	"go/token"
	"math/rand"
)

// MaxStmtDepth > 0 is currently broken because the generation of
//   _, ... = var1, ...
// statements to protect against 'unused' errors at the end of the
// function uses variables that are not in scope (they're inside inner
// blocks).
const MaxStmtDepth = 0

type StmtBuilder struct {
	rs    *rand.Rand // randomness source
	eb    *ExprBuilder
	depth int // how deep the stmt hyerarchy is

	// TODO: make currentFunc a global variable (it's needed in too
	// many places, including expr.go)
	currentFunc string // function we're building
}

func NewStmtBuilder(rs *rand.Rand) *StmtBuilder {
	sb := new(StmtBuilder)
	sb.rs = rs
	sb.eb = NewExprBuilder(rs)
	return sb
}

// holds a func Ident, and a name -> Ident map of all the variables
// declared in the body of the function
type function struct {
	name *ast.Ident
	vars map[string]*ast.Ident
}

var varNames map[string]*ast.Ident

func init() {
	varNames = make(map[string]*ast.Ident)
}

// TODO: pre-generate names and then draw them(?)
func (sb *StmtBuilder) VarIdent() *ast.Ident {
	cfn, ok := funNames[sb.currentFunc]
	if !ok {
		panic(fmt.Sprintf("currentFunc %v not found", sb.currentFunc))
	}

	// try to generate a var name until we hit one that is not already
	// in function scope
	name := fmt.Sprintf("var%v", sb.rs.Intn(1000))
	for _, ok := cfn.vars[name]; ok; {
		name = fmt.Sprintf("var%v", sb.rs.Intn(1000))
	}

	// build Ident object and return
	id := new(ast.Ident)
	id.Obj = &ast.Object{Kind: ast.Var, Name: name}
	cfn.vars[name] = id
	funNames[sb.currentFunc] = cfn

	id.Name = name
	return id
}

func (sb *StmtBuilder) Stmt() ast.Stmt {
	// Currently
	//   - Assign
	//   - Block
	nFuncs := uint32(2)

	switch sb.rs.Uint32() % nFuncs {
	case 0:
		return sb.AssignStmt()
	case 1:
		sb.depth++
		if sb.depth > MaxStmtDepth {
			return &ast.EmptyStmt{}
		}
		s := sb.BlockStmt()
		sb.depth--
		return s
	// case 2:
	// 	return sb.ExprStmt()
	default:
		panic("Stmt: bad random")
	}
}

func (sb *StmtBuilder) AssignStmt() *ast.AssignStmt {
	as := new(ast.AssignStmt)

	as.Lhs = []ast.Expr{sb.VarIdent()}
	as.Tok = token.DEFINE
	as.Rhs = []ast.Expr{sb.eb.Expr()}

	return as
}

func (sb *StmtBuilder) BlockStmt() *ast.BlockStmt {
	bs := new(ast.BlockStmt)
	stmts := []ast.Stmt{}
	for i := 0; i < 10; i++ {
		stmts = append(stmts, sb.Stmt())
	}
	bs.List = stmts

	return bs
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
// for each variable defined in sb.currentFunc
func (sb *StmtBuilder) UseVars() []ast.Stmt {
	useStmt := &ast.AssignStmt{Tok: token.ASSIGN}
	for _, name := range funNames[sb.currentFunc].vars {
		useStmt.Lhs = append(useStmt.Lhs, &ast.Ident{Name: "_"})
		useStmt.Rhs = append(useStmt.Rhs, name)
	}
	return []ast.Stmt{useStmt}
}
