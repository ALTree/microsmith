package microsmith

import (
	"go/ast"
	"go/token"
	"math"
	"math/rand"
	"strconv"
)

type ExprBuilder struct {
	rs      *rand.Rand // randomness source
	depth   int        // how deep the expr hierarchy is
	conf    ExprConf
	inScope map[Type]Scope // passed down by StmtBuilders
}

type ExprConf struct {
	// How likely it is to generate an unary expression, expressed as
	// a value in [0,1]. If 0, every expression is binary; if 1, every
	// expression is unary.
	UnaryChance float64

	// How likely it is to choose a literal (instead of a variable
	// among the ones in scope) when building an expression; expressed
	// as a value in [0,1]. If 0, only variables are chosen; if 1,
	// only literal are chosen.
	LiteralChance float64

	// How likely it is to pick a variable by indexing an array type
	// (instead of a plain variables). If 0, we never index from
	// arrays.
	IndexChance float64

	// How likely is to build a boolean binary expression by using a
	// comparison operator on non-boolean types instead of a logical
	// operator on booleans. If 0, comparison operators are never
	// used.
	ComparisonChance float64
}

func NewExprBuilder(rs *rand.Rand, conf ExprConf, inscp map[Type]Scope) *ExprBuilder {
	return &ExprBuilder{
		rs:      rs,
		conf:    conf,
		inScope: inscp,
	}
}

func (eb *ExprBuilder) chooseToken(tokens []token.Token) token.Token {
	return tokens[eb.rs.Intn(len(tokens))]
}

func (eb *ExprBuilder) BasicLit(t Type) *ast.BasicLit {
	bl := new(ast.BasicLit)

	switch t {
	case TypeInt:
		bl.Kind = token.INT
		bl.Value = strconv.Itoa(eb.rs.Intn(100))
	case TypeBool:
		panic("BasicLit: bool is not a BasicLit")
	case TypeString:
		bl.Kind = token.STRING
		bl.Value = RandString([]string{
			`"a"`, `"bb"`, `"ccc"`,
			`"dddd"`, `"eeeee"`, `"ffffff"`,
		})
	default:
		panic("BasicLit: unimplemented type " + t.String())
	}
	return bl
}

func (eb *ExprBuilder) Expr(t Type) ast.Expr {
	// Currently:
	//   - Binary
	//   - Unary
	var expr ast.Expr

	eb.depth++
	if t != TypeString && eb.rs.Float64() < eb.conf.UnaryChance {
		// there's no unary operator for strings
		expr = eb.UnaryExpr(t)
	} else {
		expr = eb.BinaryExpr(t)
	}
	eb.depth--

	return expr
}

// Returns an in-scope variable or a literal of the given kind. If
// there's no variable of the requested type in scope, returns a
// literal.
func (eb *ExprBuilder) VarOrLit(t Type) interface{} {
	// we return a literal if, either
	//   - there are no variables in scope of the type we need
	//   - the dice says "choose literal"
	if (len(eb.inScope[t]) == 0 && len(eb.inScope[t.Arr()]) == 0) ||
		eb.rs.Float64() < eb.conf.LiteralChance {
		switch t {
		case TypeInt, TypeString:
			return eb.BasicLit(t)
		case TypeBool:
			return &ast.Ident{Name: RandString([]string{"true", "false"})}
		default:
			panic("VarOrLit: unsupported type " + t.String())
		}
	}

	// if we have to return a variable, choose between a variable of
	// the given type and indexing into an array of the given type
	if (len(eb.inScope[t.Arr()]) > 0 && eb.rs.Float64() < eb.conf.IndexChance) ||
		len(eb.inScope[t]) == 0 {
		return eb.IndexExpr(t.Arr())
	}
	return RandomInScopeVar(eb.inScope[t], eb.rs)

}

// returns an IndexExpr of the given type. Panics if there's no
// indexable variables of the requsted type in scope.
// TODO: add max allowed index(?)
func (eb *ExprBuilder) IndexExpr(t Type) *ast.IndexExpr {
	inScope := eb.inScope[t]
	if len(inScope) == 0 {
		panic("IndexExpr: empty scope")
	}

	indexable := RandomInScopeVar(inScope, eb.rs)
	ie := &ast.IndexExpr{
		X: indexable,
		// no Expr for the index (for now), because constant exprs
		// that end up negative cause compilation errors.
		Index: eb.VarOrLit(TypeInt).(ast.Expr),
	}

	return ie
}

func (eb *ExprBuilder) UnaryExpr(t Type) *ast.UnaryExpr {
	ue := new(ast.UnaryExpr)

	switch t {
	case TypeInt:
		ue.Op = eb.chooseToken([]token.Token{token.ADD, token.SUB})
	case TypeBool:
		ue.Op = eb.chooseToken([]token.Token{token.NOT})
	case TypeString:
		panic("UnaryExpr: invalid kind (string)")
	default:
		panic("UnaryExpr: unimplemented type " + t.String())
	}

	// set a 0.2 chance of not generating a nested Expr, even if
	// we're not at maximum depth
	if 1/math.Pow(1.2, float64(eb.depth)) < eb.rs.Float64() {
		ue.X = eb.VarOrLit(t).(ast.Expr)
	} else {
		ue.X = eb.Expr(t)
	}

	return ue
}

func (eb *ExprBuilder) BinaryExpr(t Type) *ast.BinaryExpr {
	ue := new(ast.BinaryExpr)

	// First choose the operator...
	switch t {
	case TypeInt:
		ue.Op = eb.chooseToken([]token.Token{token.ADD, token.SUB})
	case TypeBool:
		if eb.rs.Float64() < eb.conf.ComparisonChance {
			// generate a bool expr by comparing two exprs of
			// comparable type
			ue.Op = eb.chooseToken([]token.Token{
				token.EQL, token.NEQ,
				token.LSS, token.LEQ,
				token.GTR, token.GEQ,
			})
			if ue.Op == token.EQL || ue.Op == token.NEQ {
				// every type is comparable with == and !=
				t = RandType(SupportedTypes)
			} else {
				// and these also support <, <=, >, >=
				t = RandType([]Type{TypeInt, TypeString})
			}
		} else {
			ue.Op = eb.chooseToken([]token.Token{token.LAND, token.LOR})
		}
	case TypeString:
		ue.Op = eb.chooseToken([]token.Token{token.ADD})
	default:
		panic("BinaryExpr: unimplemented type " + t.String())
	}

	// ...then build the two branches.
	if 1/math.Pow(1.2, float64(eb.depth)) < eb.rs.Float64() {
		ue.X = eb.VarOrLit(t).(ast.Expr)
		ue.Y = eb.VarOrLit(t).(ast.Expr)
	} else {
		ue.X = eb.Expr(t)
		ue.Y = eb.Expr(t)
	}

	return ue
}
