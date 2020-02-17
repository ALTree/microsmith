package microsmith

import (
	"go/ast"
	"go/token"
	"math/rand"
	"strconv"
	"strings"
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
	case "rune":
		bl.Kind = token.CHAR
		bl.Value = RandRune()
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
		bl.Value = RandString()
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
		vt, typeInScope := eb.scope.GetRandomVarOfType(t, eb.rs)
		vst, baseInScope := eb.scope.GetRandomVarOfType(t.Base(), eb.rs)

		if typeInScope && baseInScope {
			// if we can do both, choose at random
			if eb.rs.Intn(2) == 0 {
				expr = vt.Name
			} else { // take address of t.Base
				expr = &ast.UnaryExpr{
					Op: token.AND,
					X:  vst.Name, // TODO(alb): we could dereference much more complex Expr
				}
			}
		} else if typeInScope {
			expr = vt.Name
		} else if baseInScope {
			expr = &ast.UnaryExpr{
				Op: token.AND,
				X:  vst.Name, // TODO(alb): we could dereference much more complex Expr
			}
		} else {
			// nothing with type t or type t.Base is in scope, so we can't
			// return a variable nor take the address or one. Return nil.
			expr = &ast.Ident{Name: "nil"}
		}

	default:
		panic("Expr: bad type " + t.Name())
	}

	eb.depth--

	return expr
}

// VarOrLit returns either:
//   - a literal of type t
//   - an expression of type t
//
// If no expression of type t can be built, it always returns a
// literal. Otherwise, it returns a literal or an Expr with chances
// respectively (LiteralChance) and (1 - LiteralChance).
//
// When returning an expression, that can be either an ast.Ident (for
// example when t is int it could just return a variable I0 of type
// int), or a derived expression of type type. Expression are derived
// from:
//   - arrays and maps, by indexing into them
//   - channels, by receiving
//   - structs, by selecting a field
//   - pointers, by dereferencing
//
// When returning an expression, simple one are always preferred. A
// derived expression is only returned when there are not variables of
// type t in scope.
//
// TODO(alb): make it return derived Expr more often
//
// TODO(alb): we never call SliceExpr, i.e. if the requested type is
// []int we always return any []int in scope, but we should instead
// sometimes return an expr that slices into one of the []ints
func (eb *ExprBuilder) VarOrLit(t Type) interface{} {

	vt, typeInScope := eb.scope.GetRandomVarOfType(t, eb.rs)
	vst, typeCanDerive := eb.scope.GetRandomVarOfSubtype(t, eb.rs)

	// Literal of type t
	if eb.rs.Float64() < eb.conf.LiteralChance || (!typeInScope && !typeCanDerive) {
		switch t := t.(type) {
		case BasicType:
			if n := t.Name(); n == "int" || n == "string" || n == "float64" || n == "complex128" || n == "rune" {
				return eb.BasicLit(t)
			} else if n == "bool" {
				if eb.rs.Intn(2) == 0 {
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
			panic("VarOrLit: unsupported type")
		}
	}

	// Expr of type t
	if typeInScope {
		// If it's sliceable, slice it with chance 0.5
		if vt.Type.Sliceable() && eb.rs.Intn(2) == 0 {
			return eb.SliceExpr(vt)
		} else {
			return vt.Name
		}
	}
	// no variable of type t in scope, we have to derive.

	switch vst.Type.(type) {
	case ArrayType:
		return eb.ArrayIndexExpr(vst)
	case MapType:
		return eb.MapIndexExpr(vst)
	case StructType:
		return eb.StructFieldExpr(vst, t)
	case ChanType:
		return eb.ChanReceiveExpr(vst)
	case PointerType:
		return &ast.UnaryExpr{
			Op: token.MUL,
			X:  &ast.Ident{Name: vst.Name.Name},
		}
	default:
		panic("argh")
	}

	panic("unreachable")

}

// Returns an ast.IndexExpr which index into v (of type Array) either
// using an int literal or an int Expr.
func (eb *ExprBuilder) ArrayIndexExpr(v Variable) *ast.IndexExpr {
	_, ok := v.Type.(ArrayType)
	if !ok {
		panic("MakeArrayIndexExpr: not an array - " + v.String())
	}

	// We can't just generate an Expr for the index, because constant
	// exprs that end up being negative will cause a compilation error.
	//
	// If there is at least one int variable in scope, we can generate
	// 'I + Expr()' as index, which is guaranteed not to be constant. If
	// not, we just to use a literal.
	var index ast.Expr
	vi, ok := eb.scope.GetRandomVarOfType(BasicType{"int"}, eb.rs)
	if ok && eb.CanDeepen() {
		index = &ast.BinaryExpr{
			X:  vi.Name,
			Op: token.ADD,
			Y:  eb.Expr(BasicType{"int"}),
		}
	} else {
		index = eb.VarOrLit(BasicType{"int"}).(ast.Expr)
	}

	return &ast.IndexExpr{
		X:     v.Name,
		Index: index,
	}

}

// Returns an ast.IndexExpr which index into v (of type Map) either
// using a keyT literal or a KeyT Expr.
func (eb *ExprBuilder) MapIndexExpr(v Variable) *ast.IndexExpr {
	mv, ok := v.Type.(MapType)
	if !ok {
		panic("not an array - " + v.String())
	}

	var index ast.Expr
	if eb.CanDeepen() {
		index = eb.Expr(mv.KeyT)
	} else {
		index = eb.VarOrLit(mv.KeyT).(ast.Expr)
	}

	return &ast.IndexExpr{
		X:     v.Name,
		Index: index,
	}
}

// Returns an ast.SelectorExpr which select into a field of type t in
// v (of type Struct).
func (eb *ExprBuilder) StructFieldExpr(v Variable, t Type) *ast.SelectorExpr {
	sv, ok := v.Type.(StructType)
	if !ok {
		panic("not a struct - " + v.String())
	}

	for i, ft := range sv.Ftypes {
		if ft == t {
			return &ast.SelectorExpr{
				X:   v.Name,
				Sel: &ast.Ident{Name: sv.Fnames[i]},
			}
		}
	}

	panic("Could find a field of type " + t.Name() + " in struct " + v.String())
}

// Returns an ast.UnaryExpr which receive from the channel v.
func (eb *ExprBuilder) ChanReceiveExpr(v Variable) *ast.UnaryExpr {
	_, ok := v.Type.(ChanType)
	if !ok {
		panic("not a chan - " + v.String())
	}

	return &ast.UnaryExpr{
		Op: token.ARROW,
		X:  &ast.Ident{Name: v.Name.Name},
	}
}

func (eb *ExprBuilder) SliceExpr(v Variable) *ast.SliceExpr {
	if !v.Type.Sliceable() {
		panic("SliceExpr: un-sliceable type " + v.Type.Name())
	}

	var low, high ast.Expr
	indV, hasInt := eb.scope.GetRandomVarOfType(BasicType{"int"}, eb.rs)
	if hasInt && eb.CanDeepen() {
		low = &ast.BinaryExpr{
			X:  indV.Name,
			Op: token.ADD,
			Y:  eb.Expr(BasicType{"int"}),
		}
		high = &ast.BinaryExpr{
			X:  eb.Expr(BasicType{"int"}),
			Op: token.ADD,
			Y:  indV.Name,
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

	return &ast.SliceExpr{
		X:    v.Name,
		Low:  low,
		High: high,
	}
}

func (eb *ExprBuilder) UnaryExpr(t Type) *ast.UnaryExpr {
	ue := new(ast.UnaryExpr)

	// if there are pointers to t in scope, generate a t by
	// dereferencing it with chance 0.5
	if eb.rs.Intn(2) == 0 && eb.scope.HasType(PointerOf(t)) {
		ue.Op = token.MUL
		ue.X = eb.Expr(PointerOf(t))
		return ue
	}

	switch t.Name() {
	case "int", "rune", "float64", "complex128":
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
	case "int", "rune":
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
// return value of type t.
func (eb *ExprBuilder) CallExpr(t Type) *ast.CallExpr {

	// functions that are in scope and have return type t
	funcs := eb.scope.InScopeFuncs(t)

	if len(funcs) == 0 {
		// this should be handled by the caller
		panic("CallExpr: no function in scope")
	}

	// choose one of them at random
	fun := funcs[eb.rs.Intn(len(funcs))]
	switch name := fun.Name.Name; name {
	case "len":
		return eb.MakeLenCall()
	case "float64":
		ce := &ast.CallExpr{
			Fun: FloatIdent,
		}
		if eb.CanDeepen() {
			ce.Args = []ast.Expr{eb.Expr(BasicType{"int"})}
		} else {
			ce.Args = []ast.Expr{eb.VarOrLit(BasicType{"int"}).(ast.Expr)}
		}
		return ce
	case "int":
		// not enabled at the moment; see comment in NewStmtBuilder().
		panic("CallExpr: int() calls should not be generated")
	default:
		// TODO(alb): merge MakeMathCall and MakeRandCall
		if strings.HasPrefix(name, "math.") {
			return eb.MakeMathCall(fun)
		} else if strings.HasPrefix(name, "rand.") {
			return eb.MakeRandCall(fun)
		}
	}

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
		Fun: LenIdent,
	}

	if eb.CanDeepen() {
		ce.Args = []ast.Expr{eb.Expr(typ)}
	} else {
		ce.Args = []ast.Expr{eb.VarOrLit(typ).(ast.Expr)}
	}

	return ce
}

func (eb *ExprBuilder) MakeMathCall(fun Variable) *ast.CallExpr {
	ce := &ast.CallExpr{
		Fun: &ast.SelectorExpr{
			X:   &ast.Ident{Name: "math"},
			Sel: &ast.Ident{Name: fun.Name.Name[len("math."):]},
		},
	}

	cd := eb.CanDeepen()
	args := []ast.Expr{}
	for _, arg := range fun.Type.(FuncType).Args {
		if cd {
			args = append(args, eb.Expr(arg))
		} else {
			args = append(args, eb.VarOrLit(arg).(ast.Expr))
		}

	}
	ce.Args = args

	return ce
}

func (eb *ExprBuilder) MakeRandCall(fun Variable) *ast.CallExpr {
	ce := &ast.CallExpr{
		Fun: &ast.SelectorExpr{
			X:   &ast.Ident{Name: "rand"},
			Sel: &ast.Ident{Name: fun.Name.Name[len("rand."):]},
		},
	}

	cd := eb.CanDeepen()
	args := []ast.Expr{}
	for _, arg := range fun.Type.(FuncType).Args {
		if cd {
			args = append(args, eb.Expr(arg))
		} else {
			args = append(args, eb.VarOrLit(arg).(ast.Expr))
		}

	}
	ce.Args = args

	return ce
}
