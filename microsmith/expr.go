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

func NewExprBuilder(rs *rand.Rand, conf ProgramConf, s *Scope) *ExprBuilder {
	return &ExprBuilder{
		rs:    rs,
		conf:  conf,
		scope: s,
	}
}

// Returns true if the expression tree currently being built is
// allowed to become deeper.
func (eb *ExprBuilder) Deepen() bool {
	return (eb.depth <= 6) && (eb.rs.Float64() < 0.6)
}

func (eb *ExprBuilder) chooseToken(tokens []token.Token) token.Token {
	return tokens[eb.rs.Intn(len(tokens))]
}

func (eb *ExprBuilder) BasicLit(t BasicType) *ast.BasicLit {
	bl := new(ast.BasicLit)
	switch t.Name() {
	case "uint", "int", "int8", "int16", "int32":
		bl.Kind = token.INT
		bl.Value = strconv.Itoa(eb.rs.Intn(100))
	case "rune":
		bl.Kind = token.CHAR
		bl.Value = RandRune()
	case "float32", "float64":
		bl.Kind = token.FLOAT
		bl.Value = strconv.FormatFloat(999*(eb.rs.Float64()), 'f', 1, 64)
	case "complex128":
		// There's no complex basiclit, generate an IMAG
		bl.Kind = token.IMAG
		bl.Value = strconv.FormatFloat(99*(eb.rs.Float64()), 'f', 2, 64) + "i"
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
			if eb.Deepen() {
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
	eb.depth++
	defer func() { eb.depth-- }()
	if eb.depth > 32 {
		panic("eb.depth > 32")
	}

	var expr ast.Expr
	switch t := t.(type) {

	case BasicType:
		switch eb.rs.Intn(8) {
		case 0: // unary
			if t.Name() == "string" {
				expr = eb.BinaryExpr(t)
			} else {
				expr = eb.UnaryExpr(t)
			}
		case 1, 2, 3: // binary
			expr = eb.BinaryExpr(t)
		default: // function call
			if v, ok := eb.scope.GetRandomFunc(t); ok {
				expr = eb.CallExpr(v)
			} else { // fallback
				expr = eb.BinaryExpr(t)
			}
		}

	case ArrayType:
		// no unary or binary operators for composite types
		expr = eb.CompositeLit(t)

	case PointerType:
		// Either return a literal of the requested pointer type, &x
		// with x of type t.Base(), or nil.
		vt, typeInScope := eb.scope.GetRandomVarOfType(t, eb.rs)
		vst, baseInScope := eb.scope.GetRandomVarOfType(t.Base(), eb.rs)
		if typeInScope && baseInScope {
			if eb.rs.Intn(2) == 0 {
				expr = vt.Name
			} else {
				expr = &ast.UnaryExpr{
					Op: token.AND,
					X:  vst.Name,
				}
			}
		} else if typeInScope {
			expr = vt.Name
		} else if baseInScope {
			expr = &ast.UnaryExpr{
				Op: token.AND,
				X:  vst.Name,
			}
		} else {
			expr = &ast.Ident{Name: "nil"}
		}

	case FuncType:
		// We don't assign to function, so we never need to generate
		// Exprs of FuncType
		panic("Expr: called with FuncType")

	default:
		panic("Expr: bad type " + t.Name())
	}

	return expr
}

// VarOrLit returns either:
//   - a literal of type t
//   - an expression of type t
//
// If no expression of type t can be built, it always returns a
// literal. Otherwise, it returns a literal or an Expr with chances
// 0.5 - 0.5.
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
	if eb.rs.Intn(2) == 0 || (!typeInScope && !typeCanDerive) {
		switch t := t.(type) {
		case BasicType:
			switch t.Name() {
			case "bool":
				if eb.rs.Intn(2) == 0 {
					return TrueIdent
				} else {
					return FalseIdent
				}
			case "uint", "int8", "int16", "int32":
				// Since integer lits are int by default, we need an
				// explicit cast for other types.
				bl := eb.BasicLit(t)
				return &ast.CallExpr{
					Fun:  &ast.Ident{Name: t.Name()},
					Args: []ast.Expr{bl},
				}
			case "float32":
				bl := eb.BasicLit(t)
				return &ast.CallExpr{
					Fun:  &ast.Ident{Name: "float32"},
					Args: []ast.Expr{bl},
				}
			default:
				return eb.BasicLit(t)
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
	}

	panic("unreachable")
}

// Returns an ast.IndexExpr which index into v (of type Array) either
// using an int literal or an int Expr.
func (eb *ExprBuilder) ArrayIndexExpr(v Variable) *ast.IndexExpr {
	if _, ok := v.Type.(ArrayType); !ok {
		panic("ArrayIndexExpr: not an array: " + v.String())
	}

	// We can't just generate an Expr for the index, because constant
	// exprs that end up being negative cause a compilation error.
	//
	// If there is at least one int variable in scope, we can generate
	// 'I + Expr()' as index, which is guaranteed not to be constant. If
	// not, we just to use a literal.
	var index ast.Expr
	vi, ok := eb.scope.GetRandomVarOfType(BasicType{"int"}, eb.rs)
	if ok && eb.Deepen() {
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
		panic("not an array: " + v.String())
	}

	var index ast.Expr
	if eb.Deepen() {
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
		panic("not a struct: " + v.String())
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
	if _, ok := v.Type.(ChanType); !ok {
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
	if hasInt && eb.Deepen() {
		if eb.rs.Intn(8) > 0 {
			low = &ast.BinaryExpr{
				X:  indV.Name,
				Op: token.ADD,
				Y:  eb.Expr(BasicType{"int"}),
			}
		}
		if eb.rs.Intn(8) > 0 {
			high = &ast.BinaryExpr{
				X:  eb.Expr(BasicType{"int"}),
				Op: token.ADD,
				Y:  indV.Name,
			}
		}
	} else {
		if eb.rs.Intn(8) > 0 {
			low = &ast.BasicLit{
				Kind:  token.INT,
				Value: strconv.Itoa(eb.rs.Intn(8)),
			}
		}
		if eb.rs.Intn(8) > 0 {
			high = &ast.BasicLit{
				Kind:  token.INT,
				Value: strconv.Itoa(8 + eb.rs.Intn(17)),
			}
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
	case "uint":
		ue.Op = eb.chooseToken([]token.Token{token.ADD})
	case "int", "rune", "int8", "int16", "int32":
		ue.Op = eb.chooseToken([]token.Token{token.ADD, token.SUB, token.XOR})
	case "float32", "float64", "complex128":
		ue.Op = eb.chooseToken([]token.Token{token.ADD, token.SUB})
	case "bool":
		ue.Op = eb.chooseToken([]token.Token{token.NOT})
	default:
		panic("UnaryExpr: invalid type " + t.Name())
	}

	if eb.Deepen() {
		ue.X = eb.Expr(t)
	} else {
		ue.X = eb.VarOrLit(t).(ast.Expr)
	}

	return ue
}

func (eb *ExprBuilder) BinaryExpr(t Type) *ast.BinaryExpr {
	ue := new(ast.BinaryExpr)

	switch t.Name() {
	case "uint", "int8", "int16", "int32":
		ue.Op = eb.chooseToken([]token.Token{
			token.ADD, token.AND, token.AND_NOT, token.MUL,
			token.OR, token.SHR, token.SUB, token.XOR,
		})
	case "int":
		// NOTE: we can't generate token.SHR for ints, because int
		// expressions are used as args for float64() conversions, and
		// in this:
		//
		//   var i int = 2
		// 	 float64(8 >> i)
		//
		// 8 is actually of type float64(!); because, according to
		// the spec:
		//
		//   If the left operand of a non-constant shift expression is
		//   an untyped constant, it is first implicitly converted to
		//   the type it would assume if the shift expression were
		//   replaced by its left operand alone.
		//
		// and apparently in float64(8), 8 untyped is a float64. So
		//
		//   float64(8 >> i)
		//
		// fails to compile with error:
		//
		//   invalid operation: 8 >> i (shift of type float64)
		ue.Op = eb.chooseToken([]token.Token{
			token.ADD, token.AND, token.AND_NOT, token.MUL,
			token.OR /*token.SHR,*/, token.SUB, token.XOR,
		})
	case "rune":
		ue.Op = eb.chooseToken([]token.Token{
			token.ADD, token.AND, token.AND_NOT,
			token.OR, token.SUB, token.XOR,
		})
	case "float32", "float64", "complex128":
		ue.Op = eb.chooseToken([]token.Token{token.ADD, token.SUB, token.MUL})
	case "bool":
		if eb.rs.Intn(2) == 0 {
			t = RandType(eb.conf.SupportedTypes)
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

	t2 := t
	if ue.Op == token.SHR { // ensure rhs > 0 for shifts
		t2 = BasicType{"uint"}
	}

	// For some types, we need to ensure at least one leaf of the expr
	// tree is a variable, or we'll trigger compilation errors as
	// "constant overflows uint" on Exprs that end up being all
	// literals (and thus computable at compile time), and outside the
	// type's range.
	if t.Name() == "uint" || t.Name() == "int8" || t.Name() == "int16" || t.Name() == "int32" {

		// make sure LHS is not a constant
		vi, ok := eb.scope.GetRandomVarOfType(BasicType{t.Name()}, eb.rs)
		if ok {
			// at least one variable of the required type is in scope,
			// use that one
			ue.X = vi.Name
		} else {
			// otherwise, cast from an int var (there's always one)
			vi, ok := eb.scope.GetRandomVarOfType(BasicType{"int"}, eb.rs)
			if !ok {
				panic("BinaryExpr: no int in scope")
			}
			ue.X = &ast.CallExpr{
				Fun:  &ast.Ident{Name: t.Name()},
				Args: []ast.Expr{vi.Name},
			}
		}

		// RHS can be whatever
		if eb.Deepen() {
			ue.Y = eb.Expr(t2)
		} else {
			ue.Y = eb.VarOrLit(t2).(ast.Expr)
		}

		return ue
	}

	if eb.Deepen() {
		ue.X = eb.Expr(t)
		if ue.Op != token.SHR {
			ue.Y = eb.Expr(t2)
		} else {
			// The compiler rejects stupid shifts, so we need control
			// on the shift amount.
			ue.Y = eb.VarOrLit(t2).(ast.Expr)
		}
	} else {
		ue.X = eb.VarOrLit(t).(ast.Expr)
		ue.Y = eb.VarOrLit(t2).(ast.Expr)
	}

	return ue
}

// CallExpr returns a call expression with a function call that has
// return value of type t.
func (eb *ExprBuilder) CallExpr(fun Variable) *ast.CallExpr {
	name := fun.Name.Name
	switch {
	case name == "len":
		return eb.MakeLenCall()
	case name == "float64" || name == "int":
		return eb.MakeCast(fun.Type.(FuncType))
	case strings.HasPrefix(name, "math."):
		return eb.MakeMathCall(fun)
	default:
		args := make([]ast.Expr, 0, len(fun.Type.(FuncType).Args))
		for _, arg := range fun.Type.(FuncType).Args {
			args = append(args, eb.VarOrLit(arg).(ast.Expr))
		}
		return &ast.CallExpr{
			Fun:  &ast.Ident{Name: name},
			Args: args,
		}
	}
}

func (eb *ExprBuilder) MakeCast(f FuncType) *ast.CallExpr {
	ce := &ast.CallExpr{Fun: &ast.Ident{Name: f.N}}
	if eb.Deepen() {
		ce.Args = []ast.Expr{eb.Expr(f.Args[0])}
	} else {
		ce.Args = []ast.Expr{eb.VarOrLit(f.Args[0]).(ast.Expr)}
	}
	return ce

}

func (eb *ExprBuilder) MakeLenCall() *ast.CallExpr {
	// for a len call, we want a string or an array
	var typ Type
	if !IsEnabled("string", eb.conf) || eb.rs.Intn(2) == 0 {
		// choose an array of random type
		typ = ArrayType{RandType(eb.conf.SupportedTypes)}
	} else {
		// call len on string
		typ = BasicType{"string"}
	}

	ce := &ast.CallExpr{Fun: LenIdent}
	if eb.Deepen() {
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

	args := []ast.Expr{}
	for _, arg := range fun.Type.(FuncType).Args {
		if eb.Deepen() {
			args = append(args, eb.Expr(arg))
		} else {
			args = append(args, eb.VarOrLit(arg).(ast.Expr))
		}

	}
	ce.Args = args

	return ce
}
