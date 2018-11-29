package microsmith

import (
	"go/ast"
	"go/token"
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

// returns true if the expression tree currently being built is
// allowed to become deeper.
func (eb *ExprBuilder) CanDeepen() bool {
	if eb.depth > 9 {
		return false
	}

	// We want the chance of getting deeper to decrease exponentially.
	//
	// Threesholds are computed as
	//   t = (⅘)ⁿ - 0.10
	// for n in [0, 9]
	var ExprDepthChance = [10]float64{
		0.9,
		0.7,
		0.54,
		0.412,
		0.3096,
		0.22768,
		0.162144,
		0.1097152,
		0.06777216,
		0.034217728,
	}
	return eb.rs.Float64() < ExprDepthChance[eb.depth]
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
	case "complex128":
		// There's no complex basiclit, generate an IMAG
		bl.Kind = token.IMAG
		bl.Value = strconv.FormatFloat(10*(eb.rs.Float64()), 'f', 3, 64) + "i"
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
			if eb.CanDeepen() {
				clElems = append(clElems, eb.Expr(t.Base()))
			} else {
				clElems = append(clElems, eb.VarOrLit(t.Base()).(ast.Expr))
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
			if len(eb.scope.InScopeFuncs(t)) > 0 {
				expr = eb.CallExpr(t)
			} else {
				// no function in scope with return type t, fallback
				// to generating an Expr.
				if t.Name() == "string" || eb.conf.ExprConf.ExprKindChance[0] < eb.rs.Float64() {
					expr = eb.BinaryExpr(t)
				} else {
					expr = eb.UnaryExpr(t)
				}

			}
		default:
			panic("Expr: bad RandIndex value")
		}
	case ArrayType:
		// no unary or binary operators for composite types
		expr = eb.CompositeLit(t)
	case PointerType:
		if !eb.scope.TypeInScope(t.Base()) {
			//nothing is scope we could take the address of, just
			//return a nil literal
			expr = &ast.Ident{Name: "nil"}
		} else {
			expr = &ast.UnaryExpr{
				Op: token.AND,
				X:  eb.scope.RandomIdentExpr(t.Base(), eb.rs),
			}
		}
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
			if n := t.Name(); n == "int" || n == "string" || n == "float64" || n == "complex128" {
				return eb.BasicLit(t)
			} else if n == "bool" {
				if eb.rs.Int63()%2 == 0 {
					return TrueIdent
				} else {
					return FalseIdent
				}
			} else {
				panic("VarOrLit: unsupported basic type " + t.Name())
			}
		case ArrayType:
			return eb.CompositeLit(t)
		default:
			// do nothing
		}
	}

	// return a variable expression

	// index into an array of type []t
	if (eb.scope.TypeInScope(ArrOf(t)) && eb.rs.Float64() < eb.conf.IndexChance) ||
		!eb.scope.TypeInScope(t) {
		return eb.IndexExpr(ArrOf(t))
	}

	// slice expression of type t
	if t.Sliceable() && eb.rs.Float64() < eb.conf.IndexChance {
		return eb.SliceExpr(t)
	}

	return eb.scope.RandomIdentExpr(t, eb.rs)
}

// returns an IndexExpr of the given type. Panics if there's no
// indexable variables of the requsted type in scope.
// TODO: add max allowed index(?)
func (eb *ExprBuilder) IndexExpr(t Type) *ast.IndexExpr {
	if !t.Sliceable() {
		panic("IndexExpr: un-indexable type " + t.Name())
	}
	indexable := eb.scope.RandomIdentExpr(t, eb.rs)

	var index ast.Expr

	// We can't just generate an Expr for the index, because constant
	// exprs that end up negative cause compilation errors. If there's
	// at least one int variable in scope, generate 'I + Expr()',
	// which is guaranteed not to be constant. If not, just to use a
	// literal.
	if eb.scope.TypeInScope(BasicType{"int"}) &&
		eb.CanDeepen() {
		index = &ast.BinaryExpr{
			X:  eb.scope.RandomIdentExpr(BasicType{"int"}, eb.rs),
			Op: token.ADD,
			Y:  eb.Expr(BasicType{"int"}),
		}
	} else {
		index = eb.VarOrLit(BasicType{"int"}).(ast.Expr)
	}
	ie := &ast.IndexExpr{
		X:     indexable,
		Index: index,
	}

	return ie
}

// TODO: use Expr for the slice indices, not just basiclit int
func (eb *ExprBuilder) SliceExpr(t Type) *ast.SliceExpr {
	if !t.Sliceable() {
		panic("SliceExpr: un-sliceable type " + t.Name())
	}

	sliceable := eb.scope.RandomIdentExpr(t, eb.rs)
	var low, high ast.Expr

	// We can't just generate an Expr for low and high, because
	// constant exprs that end up being negative cause compilation
	// errors. If there's at least one int variable in scope, generate
	// 'I + Expr()', which is guaranteed not to be constant. If not,
	// just to use literals.
	if eb.scope.TypeInScope(BasicType{"int"}) &&
		eb.CanDeepen() {
		low = &ast.BinaryExpr{
			X:  eb.scope.RandomIdentExpr(BasicType{"int"}, eb.rs),
			Op: token.ADD,
			Y:  eb.Expr(BasicType{"int"}),
		}
		high = &ast.BinaryExpr{
			X:  eb.scope.RandomIdentExpr(BasicType{"int"}, eb.rs),
			Op: token.ADD,
			Y:  eb.Expr(BasicType{"int"}),
		}
	} else {
		low = &ast.BasicLit{
			Kind:  token.INT,
			Value: strconv.Itoa(eb.rs.Intn(8)),
		}
		high = &ast.BasicLit{
			Kind:  token.INT,
			Value: strconv.Itoa(8 + eb.rs.Intn(17)),
		}
	}

	se := &ast.SliceExpr{
		X:    sliceable,
		Low:  low,
		High: high,
	}

	return se
}

func (eb *ExprBuilder) UnaryExpr(t Type) *ast.UnaryExpr {
	ue := new(ast.UnaryExpr)

	// if there are pointers to t in scope, generate a t by
	// dereferencing it with chance 0.5
	if eb.rs.Int63()%2 == 0 && eb.scope.TypeInScope(PointerOf(t)) {
		ue.Op = token.MUL
		ue.X = eb.Expr(PointerOf(t))
		return ue
	}

	switch t.Name() {
	case "int", "float64", "complex128":
		ue.Op = eb.chooseToken([]token.Token{token.ADD, token.SUB})
	case "bool":
		ue.Op = eb.chooseToken([]token.Token{token.NOT})
	case "string":
		panic("UnaryExpr: invalid type string")
	default:
		panic("UnaryExpr: unimplemented type " + t.Name())
	}

	if eb.CanDeepen() {
		ue.X = eb.Expr(t)
	} else {
		ue.X = eb.VarOrLit(t).(ast.Expr)
	}

	return ue
}

func (eb *ExprBuilder) BinaryExpr(t Type) *ast.BinaryExpr {
	ue := new(ast.BinaryExpr)

	// First choose the operator...
	switch t.Name() {
	case "int":
		ue.Op = eb.chooseToken([]token.Token{token.ADD, token.SUB})
	case "float64", "complex128":
		ue.Op = eb.chooseToken([]token.Token{token.ADD, token.SUB, token.MUL})
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
			if name := t.Name(); name != "bool" && name != "complex128" {
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

	if eb.CanDeepen() {
		ue.X = eb.Expr(t)
		ue.Y = eb.Expr(t)
	} else {
		ue.X = eb.VarOrLit(t).(ast.Expr)
		ue.Y = eb.VarOrLit(t).(ast.Expr)
	}

	return ue
}

// CallExpr returns a call expression with a function call that has
// return value of type t. For now, we only call len
// TODO: generalize, factor out len code
func (eb *ExprBuilder) CallExpr(t Type) *ast.CallExpr {

	// functions that are in scope and have return type t
	funcs := eb.scope.InScopeFuncs(t)

	if len(funcs) == 0 {
		// this should be handled by the caller
		panic("CallExpr: no function in scope")
	}

	// choose one of them at random
	fun := funcs[eb.rs.Intn(len(funcs))]

	// len() calls are handled separately, since the argument can be
	// either an array or a string
	if fun.Name.Name == "len" {
		return eb.MakeLenCall()
	}

	if fun.Name.Name == "float64" {
		ce := &ast.CallExpr{
			Fun:  FloatIdent,
			Args: []ast.Expr{eb.Expr(BasicType{"int"})},
		}
		return ce
	}

	// not enabled at the moment; see comment in NewStmtBuilder().
	if fun.Name.Name == "int" {
		ce := &ast.CallExpr{
			Fun:  &ast.Ident{Name: "int"},
			Args: []ast.Expr{eb.Expr(BasicType{"float64"})},
		}
		return ce
	}

	// TODO(alb): handle generic function types

	panic("unreachable")
}

func (eb *ExprBuilder) MakeLenCall() *ast.CallExpr {
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
		Fun:  LenIdent,
		Args: []ast.Expr{eb.Expr(typ)},
	}
	return ce
}
