package microsmith

import (
	"go/ast"
	"go/token"
	"math/rand"
	"strconv"
)

type ExprBuilder struct {
	rs    *rand.Rand // randomness source
	depth int        // how deep the expr hyerarchy is
	conf  ExprConf
}

type ExprConf struct {
	// maximum allowed expressions nesting
	maxExprDepth int

	// Measure of how likely it is to generate an unary expression,
	// expressed as a value in [0,1]. If unaryChance is 0, every
	// expression is binary; if 1, every expression is unary.
	unaryChance float64
}

func NewExprBuilder(rs *rand.Rand) *ExprBuilder {
	return &ExprBuilder{
		rs: rs,
		conf: ExprConf{
			maxExprDepth: 2,
			unaryChance:  0.2,
		},
	}
}

func (eb *ExprBuilder) chooseToken(tokens []token.Token) token.Token {
	return tokens[eb.rs.Intn(len(tokens))]
}

func (eb *ExprBuilder) BasicLit(kind string) *ast.BasicLit {
	bl := new(ast.BasicLit)

	// TODO: generate all kinds of literal
	// kinds := []token.Token{token.INT, token.FLOAT, token.IMAG, token.CHAR, token.STRING}
	// bl.Kind = eb.chooseToken(kinds)

	switch kind {
	case "int":
		bl.Kind = token.INT
		bl.Value = strconv.Itoa(eb.rs.Intn(100))
	case "bool":
		panic("BasicLit: bool is not a BasicLit")
	default:
		panic("BasicLit: kind not implemented")
	}
	return bl
}

func (eb *ExprBuilder) Expr(kind string) ast.Expr {
	// Currently:
	//   - Binary
	//   - Unary
	var expr ast.Expr

	eb.depth++
	if eb.rs.Float64() < eb.conf.unaryChance {
		expr = eb.UnaryExpr(kind)
	} else {
		expr = eb.BinaryExpr(kind)
	}
	eb.depth--

	return expr
}

func (eb *ExprBuilder) UnaryExpr(kind string) *ast.UnaryExpr {
	ue := new(ast.UnaryExpr)

	switch kind {
	case "int":
		ue.Op = eb.chooseToken([]token.Token{token.ADD, token.SUB})
	case "bool":
		ue.Op = eb.chooseToken([]token.Token{token.NOT})
	default:
		panic("UnaryExpr: kind not implemented")
	}

	if eb.depth > eb.conf.maxExprDepth {
		// TODO: also Ident, but we don't know what Idents are in
		// scope (we don't have access to currentFunc from here).
		// Should probably make currentFunc a global variable.
		switch kind {
		case "int":
			ue.X = eb.BasicLit(kind)
		case "bool": // 'true' and 'false' are not BasicLits
			ue.X = &ast.Ident{Name: RandString(eb.rs.Int(), []string{"true", "false"})}
		}
	} else {
		ue.X = eb.Expr(kind)
	}

	return ue
}

func (eb *ExprBuilder) BinaryExpr(kind string) *ast.BinaryExpr {

	ue := new(ast.BinaryExpr)

	switch kind {
	case "int":
		ue.Op = eb.chooseToken([]token.Token{token.ADD, token.SUB})
	case "bool":
		ue.Op = eb.chooseToken([]token.Token{token.LAND, token.LOR})
	default:
		panic("UnaryExpr: kind not implemented")
	}

	if eb.depth > eb.conf.maxExprDepth {
		switch kind {
		case "int":
			ue.X = eb.BasicLit(kind)
			ue.Y = eb.BasicLit(kind)
		case "bool": // 'true' and 'false' are not BasicLits
			bools := []string{"true", "false"}
			ue.X = &ast.Ident{Name: RandString(eb.rs.Int(), bools)}
			ue.Y = &ast.Ident{Name: RandString(eb.rs.Int(), bools)}
		}
	} else {
		ue.X = eb.Expr(kind)
		ue.Y = eb.Expr(kind)
	}

	return ue
}
