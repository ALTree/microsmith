package microsmith

import (
	"go/ast"
	"go/token"
	"math/rand"
	"strconv"
)

const MaxExprDepth = 3

type ExprBuilder struct {
	rs    *rand.Rand // randomness source
	depth int        // how deep the expr hyerarchy is
}

func NewExprBuilder(rs *rand.Rand) *ExprBuilder {
	return &ExprBuilder{rs: rs}
}

func (eb *ExprBuilder) chooseToken(tokens []token.Token) token.Token {
	return tokens[eb.rs.Intn(len(tokens))]
}

func (eb *ExprBuilder) BasicLit() *ast.BasicLit {
	bl := new(ast.BasicLit)

	// TODO: generate all kinds of literal

	// kinds := []token.Token{token.INT, token.FLOAT, token.IMAG, token.CHAR, token.STRING}
	// bl.Kind = eb.chooseToken(kinds)

	bl.Kind = token.INT
	bl.Value = strconv.Itoa(eb.rs.Intn(100))

	return bl
}

func (eb *ExprBuilder) Expr() ast.Expr {
	// Currently:
	//   - Binary
	//   - Unary
	var expr ast.Expr

	eb.depth++
	if eb.rs.Uint32()%10 < 4 { // TODO: use constants, or make it configurable
		expr = eb.UnaryExpr()
	} else {
		expr = eb.BinaryExpr()
	}
	eb.depth--

	return expr
}

func (eb *ExprBuilder) UnaryExpr() *ast.UnaryExpr {
	// + - ! ^ * & <-
	unaryOps := []token.Token{
		token.ADD,
		token.SUB,
		// token.NOT,
		// token.XOR,
		// token.MUL,
		// token.AND,
		// token.ARROW,
	}

	ue := new(ast.UnaryExpr)
	ue.Op = eb.chooseToken(unaryOps)

	if eb.depth > MaxExprDepth {
		// TODO: also Ident, but we don't know what Idents are in
		// scope (we don't have access to currentFunc from here).
		// Should probably make currentFunc a global variable.
		ue.X = eb.BasicLit()
	} else {
		ue.X = eb.Expr()
	}

	return ue
}

func (eb *ExprBuilder) BinaryExpr() *ast.BinaryExpr {
	// TODO: add more
	binaryOps := []token.Token{
		token.ADD,
		token.SUB,
		// token.NOT,
		// token.XOR,
		token.MUL,
		//token.QUO,
		// token.AND,
		// token.ARROW,
	}

	ue := new(ast.BinaryExpr)
	ue.Op = eb.chooseToken(binaryOps)

	if eb.depth > MaxExprDepth {
		ue.X = eb.BasicLit()
		ue.Y = eb.BasicLit()
	} else {
		ue.X = eb.Expr()
		ue.Y = eb.Expr()
	}

	return ue
}
