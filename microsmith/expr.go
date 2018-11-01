package microsmith

import (
	"go/ast"
	"go/token"
	"math"
	"math/rand"
	"strconv"
)

type ExprBuilder struct {
	rs    *rand.Rand // randomness source
	depth int        // how deep the expr hierarchy is
	conf  ProgramConf
	scope *Scope // passed down by StmtBuilders
}

type ExprConf struct {

	// When building a general Expr, chances of generating, in order:
	//  0. Unary Expression
	//  1. Binary Expression
	//  2. Function call
	ExprKindChance []float64

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

func NewExprBuilder(rs *rand.Rand, conf ProgramConf, s *Scope) *ExprBuilder {
	return &ExprBuilder{
		rs:    rs,
		conf:  conf,
		scope: s,
	}
}

func (eb *ExprBuilder) chooseToken(tokens []token.Token) token.Token {
	return tokens[eb.rs.Intn(len(tokens))]
}

func (eb *ExprBuilder) BasicLit(t Type) *ast.BasicLit {
	bl := new(ast.BasicLit)

	switch t.Name() {
	case "int":
		bl.Kind = token.INT
		bl.Value = strconv.Itoa(eb.rs.Intn(100))
	case "float64":
		bl.Kind = token.FLOAT
		bl.Value = strconv.FormatFloat(100*(eb.rs.Float64()), 'f', 3, 64)
	case "bool":
		panic("BasicLit: bool is not a BasicLit")
	case "string":
		bl.Kind = token.STRING
		bl.Value = RandString([]string{
			`"a"`, `"bb"`, `"ccc"`,
			`"dddd"`, `"eeeee"`, `"ffffff"`,
		})
	default:
		panic("BasicLit: unimplemented type " + t.Name())
	}

	return bl
}

func (eb *ExprBuilder) CompositeLit(t Type) *ast.CompositeLit {
	switch t := t.(type) {
	case BasicType:
		panic("CompositeLit: basic type " + t.Name())
	case ArrayType:
		cl := &ast.CompositeLit{
			Type: &ast.ArrayType{Elt: &ast.Ident{
				Name: t.Base().Name()},
			},
		}
		clElems := []ast.Expr{}
		for i := 0; i < eb.rs.Intn(5); i++ {
			if 1/math.Pow(1.2, float64(eb.depth)) < eb.rs.Float64() {
				clElems = append(clElems, eb.VarOrLit(t.Base()).(ast.Expr))
			} else {
				clElems = append(clElems, eb.Expr(t.Base()))
			}
		}
		cl.Elts = clElems

		return cl
	default:
		panic("CompositeLit: bad type " + t.Name())
	}
}

func (eb *ExprBuilder) Expr(t Type) ast.Expr {
	// Currently:
	//   - Unary
	//   - Binary
	//   - CompositeLit
	//   - Call
	// TODO:
	//   - SimpleLit
	var expr ast.Expr

	eb.depth++
	if eb.depth > 128 {
		panic("eb.depth > 128")
	}

	switch t := t.(type) {
	case BasicType:
		switch RandIndex(eb.conf.ExprKindChance, eb.rs.Float64()) {
		case 0: // unary
			if t.Name() == "string" {
				// no unary operator for strings, return a binary expr
				expr = eb.BinaryExpr(t)
			} else {
				expr = eb.UnaryExpr(t)
			}
		case 1: // binary
			expr = eb.BinaryExpr(t)
		case 2: // function call
			if t.Name() == "int" {
				// only have len for now
				expr = eb.CallExpr(t)
			} else {
				expr = eb.BinaryExpr(t)
			}
		default:
			panic("Expr: bad RandIndex value")
		}
	case ArrayType:
		// no unary or binary operators for composite types
		expr = eb.CompositeLit(t)
	default:
		panic("Expr: bad type " + t.Name())
	}
	eb.depth--

	return expr
}

// Returns an in-scope variable or a literal of the given kind. If
// there's no variable of the requested type in scope, returns a
// literal.
func (eb *ExprBuilder) VarOrLit(t Type) interface{} {
	// return a literal
	if (!eb.scope.TypeInScope(t) && !eb.scope.TypeInScope(ArrOf(t))) ||
		eb.rs.Float64() < eb.conf.LiteralChance {
		switch t := t.(type) {
		case BasicType:
			if n := t.Name(); n == "int" || n == "string" || n == "float64" {
				return eb.BasicLit(t)
			} else if n == "bool" {
				return &ast.Ident{Name: RandString([]string{"true", "false"})}
			} else {
				panic("VarOrLit: unsupported basic type " + t.Name())
			}
		case ArrayType:
			return eb.CompositeLit(t)
		}
	}

	// return a variable

	// index into an array of type []t
	if (eb.scope.TypeInScope(ArrOf(t)) && eb.rs.Float64() < eb.conf.IndexChance) ||
		!eb.scope.TypeInScope(t) {
		return eb.IndexExpr(ArrOf(t))
	}

	// slice expression of type t
	if t.Sliceable() && eb.rs.Float64() < eb.conf.IndexChance {
		return eb.SliceExpr(t)
	}

	return eb.scope.RandomIdent(t, eb.rs)
}

// returns an IndexExpr of the given type. Panics if there's no
// indexable variables of the requsted type in scope.
// TODO: add max allowed index(?)
func (eb *ExprBuilder) IndexExpr(t Type) *ast.IndexExpr {
	indexable := eb.scope.RandomIdent(t, eb.rs)
	ie := &ast.IndexExpr{
		X: indexable,
		// no Expr for the index (for now), because constant exprs
		// that end up negative cause compilation errors.
		Index: eb.VarOrLit(BasicType{"int"}).(ast.Expr),
	}

	return ie
}

// TODO: use Expr for the slice indices, not just basiclit int
func (eb *ExprBuilder) SliceExpr(t Type) *ast.SliceExpr {
	if !t.Sliceable() {
		panic("SliceExpr: un-sliceable type " + t.Name())
	}

	sliceable := eb.scope.RandomIdent(t, eb.rs)
	se := &ast.SliceExpr{
		X: sliceable,
		Low: &ast.BasicLit{
			Kind:  token.INT,
			Value: strconv.Itoa(eb.rs.Intn(9)),
		},
		High: &ast.BasicLit{
			Kind:  token.INT,
			Value: strconv.Itoa(8 + eb.rs.Intn(17)),
		},
	}

	return se
}

func (eb *ExprBuilder) UnaryExpr(t Type) *ast.UnaryExpr {
	ue := new(ast.UnaryExpr)

	switch t.Name() {
	case "int", "float64":
		ue.Op = eb.chooseToken([]token.Token{token.ADD, token.SUB})
	case "bool":
		ue.Op = eb.chooseToken([]token.Token{token.NOT})
	case "string":
		panic("UnaryExpr: invalid type string")
	default:
		panic("UnaryExpr: unimplemented type " + t.Name())
	}

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
	switch t.Name() {
	case "int", "float64":
		ue.Op = eb.chooseToken([]token.Token{token.ADD, token.SUB})
	case "bool":
		if eb.rs.Float64() < eb.conf.ComparisonChance {
			// When requested a bool, we can generate a comparison
			// between any two other types (among the enabled ones).
			// If ints and/or strings are enabled, we can generate
			// '<' & co., otherwise we're restricted to eq/neq.

			// first, choose a random type
			t = RandType(eb.conf.SupportedTypes)

			// now find a suitable op
			ops := []token.Token{token.EQL, token.NEQ}
			if name := t.Name(); name == "int" || name == "string" {
				ops = append(ops, []token.Token{
					token.LSS, token.LEQ,
					token.GTR, token.GEQ,
				}...)
			}
			ue.Op = eb.chooseToken(ops)
		} else {
			ue.Op = eb.chooseToken([]token.Token{token.LAND, token.LOR})
		}
	case "string":
		ue.Op = eb.chooseToken([]token.Token{token.ADD})
	default:
		panic("BinaryExpr: unimplemented type " + t.Name())
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

// CallExpr returns a call expression with a function call that has
// return value of type t. For now, we only call len
// TODO: generalize, factor out len code
func (eb *ExprBuilder) CallExpr(t Type) *ast.CallExpr {
	switch t := t.(type) {
	case BasicType:
		if t.Name() == "int" {
			// for a len call, we want a string or an array
			var typ Type
			if !IsEnabled("string", eb.conf) || eb.rs.Float64() < 0.5 {
				// choose an array of random type
				typ = ArrayType{RandType(eb.conf.SupportedTypes)}
			} else {
				// call len on string
				typ = BasicType{"string"}
			}
			ce := &ast.CallExpr{
				Fun: &ast.Ident{Name: LenFun.Name()},
				Args: []ast.Expr{
					// TODO: why not Expr?
					eb.VarOrLit(typ).(ast.Expr),
				},
			}
			return ce
		} else {
			panic("CallExpr: unimplemented type " + t.Name())
		}
	default:
		panic("CallExpr: unimplemented type " + t.Name())
	}
}
