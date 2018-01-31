package microsmith

import (
	"go/ast"
	"go/token"
	"math/rand"
	"strconv"
)

type ExprBuilder struct {
	rs      *rand.Rand // randomness source
	depth   int        // how deep the expr hierarchy is
	conf    ExprConf
	inScope map[string]Scope // passed down by StmtBuilders
}

type ExprConf struct {
	// maximum allowed expressions nesting
	maxExprDepth int

	// How likely it is to generate an unary expression, expressed as
	// a value in [0,1]. If 0, every expression is binary; if 1, every
	// expression is unary.
	unaryChance float64

	// How likely it is to choose a literal (instead of a variable
	// among the ones in scope) when building an expression; expressed
	// as a value in [0,1]. IF 0, only variables are chosen; if 1,
	// only literal are chosen.
	literalChance float64

	// How likely is to build a boolean binary expression by using a
	// comparison operator on non-boolean types instead of a logical
	// operator on booleans. If 0, comparison operators are never
	// used.
	comparisonChance float64
}

func NewExprBuilder(rs *rand.Rand, inscp map[string]Scope) *ExprBuilder {
	return &ExprBuilder{
		rs: rs,
		conf: ExprConf{
			maxExprDepth:     5,
			unaryChance:      0.1,
			literalChance:    0.2,
			comparisonChance: 0.2,
		},
		inScope: inscp,
	}
}

func (eb *ExprBuilder) chooseToken(tokens []token.Token) token.Token {
	return tokens[eb.rs.Intn(len(tokens))]
}

func (eb *ExprBuilder) BasicLit(kind string) *ast.BasicLit {
	bl := new(ast.BasicLit)

	switch kind {
	case "int":
		bl.Kind = token.INT
		bl.Value = strconv.Itoa(eb.rs.Intn(100))
	case "bool":
		panic("BasicLit: bool is not a BasicLit")
	case "string":
		bl.Kind = token.STRING
		bl.Value = RandString([]string{
			`"a"`, `"bb"`, `"ccc"`,
			`"dddd"`, `"eeeee"`, `"ffffff"`,
		})
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
	if !(kind == "string" || kind == "intArr") && eb.rs.Float64() < eb.conf.unaryChance {
		// there's no unary operator for strings
		expr = eb.UnaryExpr(kind)
	} else {
		expr = eb.BinaryExpr(kind)
	}
	eb.depth--

	return expr
}

// Returns an in-scope variable or a literal of the given kind. If
// there's no variable of the requested type in scope, returns a
// literal.
func (eb *ExprBuilder) VarOrLit(kind string) interface{} {
	if (len(eb.inScope[kind]) == 0 && len(eb.inScope[kind+"Arr"]) == 0) || eb.rs.Float64() < eb.conf.literalChance {
		switch kind {
		case "int", "string":
			return eb.BasicLit(kind)
		case "bool":
			return &ast.Ident{Name: RandString([]string{"true", "false"})}
		default:
			panic("VarOrLit: unsupported type")
		}
	}

	if len(eb.inScope[kind+"Arr"]) > 0 && eb.rs.Float64() < 0.2 {
		return eb.IndexExpr(kind)
	}
	return RandomInScopeVar(eb.inScope[kind], eb.rs)

}

// returns an IndexExpr of the given type. Panics if there's no
// indexable variables of the requsted type in scope.
// TODO: add max allowed index(?)
func (eb *ExprBuilder) IndexExpr(kind string) *ast.IndexExpr {
	inScope := eb.inScope[kind+"Arr"]
	if len(inScope) == 0 {
		panic("IndexExpr: empty scope")
	}

	// always use index 42 for now, for debugging purpose
	indexable := RandomInScopeVar(inScope, eb.rs)

	ie := &ast.IndexExpr{
		X:     indexable,
		Index: &ast.BasicLit{Kind: token.INT, Value: "42"},
	}

	return ie
}

func (eb *ExprBuilder) UnaryExpr(kind string) *ast.UnaryExpr {
	ue := new(ast.UnaryExpr)

	switch kind {
	case "int":
		ue.Op = eb.chooseToken([]token.Token{token.ADD, token.SUB})
	case "bool":
		ue.Op = eb.chooseToken([]token.Token{token.NOT})
	case "string":
		panic("UnaryExpr: invalid kind (string)")
	default:
		panic("UnaryExpr: kind not implemented")
	}

	// set a 0.2 chance of not generating a nested Expr, even if
	// we're not at maximum depth
	if eb.rs.Float64() < 0.2 || eb.depth > eb.conf.maxExprDepth {
		ue.X = eb.VarOrLit(kind).(ast.Expr)
	} else {
		ue.X = eb.Expr(kind)
	}

	return ue
}

func (eb *ExprBuilder) BinaryExpr(kind string) *ast.BinaryExpr {
	ue := new(ast.BinaryExpr)

	// First choose the operator...
	switch kind {
	case "int":
		ue.Op = eb.chooseToken([]token.Token{token.ADD, token.SUB})
	case "bool":
		if eb.rs.Float64() < eb.conf.comparisonChance {
			// generate a bool expr by comparing two exprs of
			// comparable type
			ue.Op = eb.chooseToken([]token.Token{
				token.EQL, token.NEQ,
				token.LSS, token.LEQ,
				token.GTR, token.GEQ,
			})
			if ue.Op == token.EQL || ue.Op == token.NEQ {
				// every type is comparable with == and !=
				kind = RandString(SupportedTypes)
			} else {
				// and these also support <, <=, >, >=
				kind = RandString([]string{"int", "string"})
			}
		} else {
			ue.Op = eb.chooseToken([]token.Token{token.LAND, token.LOR})
		}
	case "string":
		ue.Op = eb.chooseToken([]token.Token{token.ADD})
	default:
		panic("BinaryExpr: kind not implemented")
	}

	// ...then build the two branches.
	// There's a 0.2 chance of not generating a nested Expr, even if
	// we're not at maximum depth
	if eb.rs.Float64() < 0.2 || eb.depth > eb.conf.maxExprDepth {
		ue.X = eb.VarOrLit(kind).(ast.Expr)
		ue.Y = eb.VarOrLit(kind).(ast.Expr)
	} else {
		ue.X = eb.Expr(kind)
		ue.Y = eb.Expr(kind)
	}

	return ue
}
