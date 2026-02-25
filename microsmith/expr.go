package microsmith

import (
	"go/ast"
	"go/token"
	"math/rand"
	"strconv"
	"strings"
)

// --------------------------------
//  ExprBuilder Type
// --------------------------------

type ExprBuilder struct {
	pb *PackageBuilder
	C  *Context
	R  *rand.Rand
	S  *Scope

	depth int // how deep the expr hierarchy is
}

func NewExprBuilder(pb *PackageBuilder) *ExprBuilder {
	return &ExprBuilder{
		pb: pb,
		C:  pb.ctx,
		R:  pb.rs,
		S:  pb.ctx.scope,
	}
}

// --------------------------------
//  Builder Methods
// --------------------------------

// Returns true if the expression tree currently being built is
// allowed to become deeper.
func (eb *ExprBuilder) Deepen() bool {
	return (eb.depth <= 6) && (eb.R.Float64() < 0.7)
}

func (eb *ExprBuilder) BasicLit(t BasicType) ast.Expr {
	bl := new(ast.BasicLit)
	switch t.Name() {
	case "byte", "uint8", "uint16", "uint32", "uint64", "uint", "uintptr", "int", "int8", "int16", "int32", "int64", "any":
		bl.Kind = token.INT
		bl.Value = strconv.Itoa(eb.R.Intn(100))
	case "rune":
		bl.Kind = token.CHAR
		bl.Value = RandRune()
	case "float32", "float64":
		bl.Kind = token.FLOAT
		bl.Value = strconv.FormatFloat(1e4*(eb.R.Float64()), 'f', 1, 64)
	case "complex128":
		// There's no complex basiclit, generate an IMAG
		bl.Kind = token.IMAG
		bl.Value = strconv.FormatFloat(1e4*(eb.R.Float64()), 'f', 2, 64) + "i"
	case "bool":
		if eb.R.Intn(2) == 0 {
			return TrueIdent
		} else {
			return FalseIdent
		}
	case "string":
		bl.Kind = token.STRING
		bl.Value = RandString()
	default:
		panic("Unimplemented for " + t.Name())
	}

	return bl
}

func (eb *ExprBuilder) CompositeLit(t Type) *ast.CompositeLit {
	switch t := t.(type) {
	case BasicType:
		panic("No CompositeLit of type " + t.Name())
	case ArrayType:
		cl := &ast.CompositeLit{Type: t.Ast()}
		elems := []ast.Expr{}
		for i := 0; i < min(t.Len, 3); i++ {
			if eb.Deepen() {
				elems = append(elems, eb.Expr(t.Base()))
			} else {
				elems = append(elems, eb.VarOrLit(t.Base()))
			}
		}
		cl.Elts = elems
		return cl
	case SliceType:
		cl := &ast.CompositeLit{Type: t.Ast()}
		elems := []ast.Expr{}
		if eb.R.Intn(4) > 0 { // plain array literal
			for i := 0; i < eb.R.Intn(5); i++ {
				if eb.Deepen() {
					elems = append(elems, eb.Expr(t.Base()))
				} else {
					elems = append(elems, eb.VarOrLit(t.Base()))
				}
			}
		} else { // keyed literals
			if eb.Deepen() {
				elems = append(elems, &ast.KeyValueExpr{
					Key:   eb.BasicLit(BT{N: "int"}),
					Value: eb.Expr(t.Base()),
				})
			} else {
				elems = append(elems, &ast.KeyValueExpr{
					Key:   eb.BasicLit(BT{N: "int"}),
					Value: eb.VarOrLit(t.Base()),
				})
			}
		}
		cl.Elts = elems
		return cl
	case MapType:
		cl := &ast.CompositeLit{Type: t.Ast()}
		if eb.Deepen() {
			// since Expr as allowed as key, we can generate
			// non-constant expr which guarantees no "duplicate key"
			// compilation error.
			for i := 0; i < eb.R.Intn(4); i++ {
				ek, ok := eb.NonConstantExpr(t.KeyT)
				cl.Elts = append(cl.Elts, &ast.KeyValueExpr{Key: ek, Value: eb.Expr(t.ValueT)})
				if !ok {
					break
				}
			}
		} else {
			// No way to guarantee uniqueness, only generate one pair.
			cl.Elts = append(cl.Elts, &ast.KeyValueExpr{
				Key:   eb.VarOrLit(t.KeyT),
				Value: eb.VarOrLit(t.ValueT),
			})
		}
		return cl
	case StructType:
		cl := &ast.CompositeLit{Type: t.Ast()}
		elems := []ast.Expr{}
		for _, t := range t.Ftypes {
			if eb.Deepen() {
				elems = append(elems, eb.Expr(t))
			} else {
				elems = append(elems, eb.VarOrLit(t))
			}
		}
		cl.Elts = elems
		return cl
	default:
		panic("CompositeLit: unsupported type " + t.Name())
	}
}

func (eb *ExprBuilder) TypeParamLit(t TypeParam) ast.Expr {
	lit := &ast.BasicLit{Kind: token.INT, Value: "77"}
	return &ast.CallExpr{
		Fun:  t.Ast(),
		Args: []ast.Expr{lit},
	}
}

func (eb *ExprBuilder) Expr(t Type) ast.Expr {
	eb.depth++
	defer func() { eb.depth-- }()

	if eb.R.Intn(8) == 0 {
		return eb.RandCallExpr(t)
	}

	switch t := t.(type) {

	case BasicType, TypeParam:
		switch eb.R.Intn(7) {
		case 0:
			if bt, ok := t.(BasicType); ok {
				return eb.Cast(bt)
			}
			fallthrough
		case 1, 2, 3:
			return eb.UnaryExpr(t)
		case 4, 5, 6:
			return eb.BinaryExpr(t)
		default:
			panic("unreachable")
		}

	case SliceType:
		if t.Etype.Name() == "byte" && eb.R.Intn(3) == 0 {
			var arg ast.Expr
			if eb.Deepen() {
				arg = eb.Expr(BT{"string"})
			} else {
				arg = eb.VarOrLit(BT{"string"})
			}
			return &ast.CallExpr{
				Fun:  &ast.Ident{Name: t.Name()},
				Args: []ast.Expr{arg},
			}
		}

		if eb.R.Intn(2) == 0 {
			return eb.MakeAppendCall(t)
		}
		return eb.VarOrLit(t)

	case ArrayType, ChanType, FuncType, MapType, StructType, ExternalType:
		return eb.VarOrLit(t)

	case InterfaceType:
		return CastToType(t, &ast.Ident{Name: "nil"})

	case PointerType:
		// 50/50 between:
		//  - a new() call
		//  - a variable of type t, an unary &t.Base(), or a typed nil
		if eb.R.Intn(2) == 0 {
			ce := &ast.CallExpr{Fun: &ast.Ident{Name: "new"}}
			if eb.Deepen() {
				ce.Args = []ast.Expr{eb.Expr(t.Base())}
			} else {
				ce.Args = []ast.Expr{eb.VarOrLit(t.Base())}
			}
			return ce
		}

		vt, typeInScope := eb.S.RandVar(t)
		vst, baseInScope := eb.S.RandVar(t.Base())
		switch n := eb.R.Intn(3); {
		case n == 0 && typeInScope:
			return vt.Name
		case n == 1 && baseInScope:
			return &ast.UnaryExpr{Op: token.AND, X: vst.Name}
		default:
			return CastToType(t, &ast.Ident{Name: "nil"})
		}

	default:
		panic("Unimplemented type " + t.Name())
	}
}

// NonConstantExpr is like Expr(), except it either returns a non
// constant expression (and true), or signals failure to do so by
// returning false as the second value.
func (eb *ExprBuilder) NonConstantExpr(t Type) (ast.Expr, bool) {
	eb.depth++
	defer func() { eb.depth-- }()

	switch t := t.(type) {
	case BasicType:
		v, ok := eb.S.RandVar(t)
		if !ok {
			return eb.Expr(t), false
		}
		op := RandItem(eb.R, BinOps(t))
		// Reject shifts to avoid "negative shift count" errors. In
		// other places we handle the issue by forcing the RHS to be
		// an uint, but here we don't want to change the operand's
		// types. Also reject divisions to avoid "division by zero"
		// errors.
		if op == token.SHL || op == token.SHR || op == token.REM || op == token.QUO {
			op = token.ADD
		}
		return &ast.BinaryExpr{
			X:  v.Name,
			Op: op,
			Y:  eb.Expr(t),
		}, true
	default:
		return eb.Expr(t), true
	}
}

func (eb *ExprBuilder) VarOrLit(t Type) ast.Expr {
	// If t is a type parameter, 50/50 between returning a variable
	// and building a literal; except for type parameters that don't
	// allow literals (like interface { int | []int }); for those it's
	// always a variable.
	if tp, ok := t.(TypeParam); ok {
		if tp.HasLiterals() && eb.R.Intn(2) == 0 {
			return eb.TypeParamLit(tp)
		}
		if v, ok := eb.S.RandVar(t); !ok {
			panic("VarOrLit couldn't find a type parameter variable")
		} else {
			return v.Name
		}
	}

	if et, ok := t.(ExternalType); ok {
		return et.Build()
	}

	vst, typeCanDerive := eb.S.RandVarSubType(t)
	if !typeCanDerive || !eb.Deepen() {
		switch t := t.(type) {
		case BasicType:
			bl := eb.BasicLit(t)
			if t.NeedsCast() {
				bl = CastToType(t, bl)
			}
			return bl
		case SliceType, MapType:
			if eb.R.Intn(3) == 0 {
				return eb.MakeMakeCall(t)
			} else {
				return eb.CompositeLit(t)
			}
		case ArrayType, StructType:
			return eb.CompositeLit(t)
		case ChanType:
			// No literal of type Chan, but we can return make(chan t)
			return &ast.CallExpr{
				Fun: &ast.Ident{Name: "make"},
				Args: []ast.Expr{
					&ast.ChanType{Dir: 3, Value: t.Base().Ast()},
				},
			}
		case PointerType, FuncType, InterfaceType:
			return CastToType(t, &ast.Ident{Name: "nil"})
		case TypeParam:
			return eb.TypeParamLit(t)
		default:
			panic("unhandled type " + t.Name())
		}
	}

	// Slice, once in a while.
	if _, ok := vst.Type.(SliceType); ok {
		if t.Equal(vst.Type) && eb.R.Intn(4) == 0 {
			return eb.SliceExpr(vst)
		}
	}
	if bt, ok := vst.Type.(BasicType); ok && bt.N == "string" {
		if t.Equal(vst.Type) && eb.R.Intn(4) == 0 {
			return eb.SliceExpr(vst)
		}
	}

	return eb.SubTypeExpr(vst.Name, vst.Type, t)
}

// e: the Expr being built
// t: the current type of e
// target: the target type of e
func (eb *ExprBuilder) SubTypeExpr(e ast.Expr, t, target Type) ast.Expr {
	if t.Equal(target) {
		return e
	}

	eb.depth++
	defer func() { eb.depth-- }()

	switch t := t.(type) {
	case ArrayType:
		return eb.SubTypeExpr(eb.IndexExpr(e, true), t.Base(), target)
	case SliceType:
		return eb.SubTypeExpr(eb.IndexExpr(e), t.Base(), target)
	case BasicType:
		panic("basic types should not get here")
	case ChanType:
		return eb.SubTypeExpr(eb.ChanReceiveExpr(e), t.Base(), target)
	case MapType:
		return eb.SubTypeExpr(eb.MapIndexExpr(e, t.KeyT), t.ValueT, target)
	case PointerType:
		return eb.SubTypeExpr(eb.StarExpr(e), t.Base(), target)
	case StructType:
		return eb.SubTypeExpr(eb.StructFieldExpr(e, t, target), target, target)
	case FuncType:
		return eb.SubTypeExpr(eb.CallExpr(e, t.Args), t.Ret[0], target)
	case InterfaceType:
		return eb.SubTypeExpr(eb.MethodExpr(e, t, target), target, target)
	default:
		panic("unhandled type " + t.Name())
	}
}

// Returns e(...)
func (eb *ExprBuilder) CallExpr(e ast.Expr, at []Type) *ast.CallExpr {
	var args []ast.Expr
	for _, a := range at {
		t := a
		if arg, ok := (a).(EllipsisType); ok {
			t = arg.Base
		}
		if eb.R.Intn(2) == 0 && eb.Deepen() {
			args = append(args, eb.Expr(t))
		} else {
			args = append(args, eb.VarOrLit(t))
		}
	}
	return &ast.CallExpr{Fun: e, Args: args}
}

// Returns v.M(...)
func (eb *ExprBuilder) MethodExpr(e ast.Expr, t InterfaceType, target Type) ast.Expr {
	for _, m := range t.Methods {
		if m.Func.Contains(target) {
			sl := &ast.SelectorExpr{X: e, Sel: m.Name}
			if m.Func.Equal(target) {
				return sl
			} else {
				return eb.CallExpr(sl, m.Func.Args)
			}
		}
	}

	panic("unreachable:" + t.Name() + " " + " target: " + target.Name())
}

// Returns e[...]
func (eb *ExprBuilder) IndexExpr(e ast.Expr, nonconst ...bool) *ast.IndexExpr {
	var i ast.Expr
	if (eb.R.Intn(2) == 0 && eb.Deepen()) ||
		(len(nonconst) > 0 && nonconst[0] == true) {
		i, _ = eb.NonConstantExpr(BT{"int"})
	} else {
		i = eb.VarOrLit(BT{"int"})
	}

	return &ast.IndexExpr{X: e, Index: i}
}

// Returns *e
func (eb *ExprBuilder) StarExpr(e ast.Expr) *ast.UnaryExpr {
	return &ast.UnaryExpr{Op: token.MUL, X: e}
}

// Returns e[k] with e map and k of type t
func (eb *ExprBuilder) MapIndexExpr(e ast.Expr, t Type) *ast.IndexExpr {
	var i ast.Expr
	if eb.Deepen() {
		i = eb.Expr(t)
	} else {
		i = eb.VarOrLit(t)
	}

	return &ast.IndexExpr{X: e, Index: i}
}

// Returns e.<...> where the whole expression has type target.
func (eb *ExprBuilder) StructFieldExpr(e ast.Expr, t StructType, target Type) ast.Expr {
	for i, ft := range t.Ftypes {
		if ft.Contains(target) {
			sl := &ast.SelectorExpr{
				X:   e,
				Sel: &ast.Ident{Name: t.Fnames[i]},
			}
			return eb.SubTypeExpr(sl, ft, target)
		}
	}
	panic("unreachable:" + t.Name() + " " + " target: " + target.Name())
}

// Returns <-e
func (eb *ExprBuilder) ChanReceiveExpr(e ast.Expr) *ast.UnaryExpr {
	return &ast.UnaryExpr{Op: token.ARROW, X: e}
}

func (eb *ExprBuilder) SliceExpr(v Variable) *ast.SliceExpr {
	if !v.Type.Sliceable() {
		panic("Cannot slice type " + v.Type.Name())
	}

	var low, high ast.Expr
	var ok bool
	if eb.Deepen() {
		if eb.R.Intn(8) > 0 {
			low, ok = eb.NonConstantExpr(BT{"int"})
			if !ok {
				panic("NonConstantExpr of int failed")
			}
		}
		if eb.R.Intn(8) > 0 {
			high, ok = eb.NonConstantExpr(BT{"int"})
			if !ok {
				panic("NonConstantExpr of int failed")
			}
		}
	} else {
		if eb.R.Intn(8) > 0 {
			low = &ast.BasicLit{
				Kind:  token.INT,
				Value: strconv.Itoa(eb.R.Intn(8)),
			}
		}
		if eb.R.Intn(8) > 0 {
			high = &ast.BasicLit{
				Kind:  token.INT,
				Value: strconv.Itoa(8 + eb.R.Intn(17)),
			}
		}
	}

	return &ast.SliceExpr{
		X:    v.Name,
		Low:  low,
		High: high,
	}
}

// returns an *ast.UnaryExpr of type t, or a VarOrLit as fallback if
// type t has no unary operators.
func (eb *ExprBuilder) UnaryExpr(t Type) ast.Expr {
	ue := new(ast.UnaryExpr)

	// if there are pointers to t in scope, generate a t by
	// dereferencing it with chance 0.5
	if eb.R.Intn(2) == 0 && eb.S.Has(PointerOf(t)) {
		ue.Op = token.MUL
		// See comment in Expr() for PointerType on why we must call
		// Expr() here.
		ue.X = eb.Expr(PointerOf(t))
		return ue
	}

	if ops := UnaryOps(t); len(ops) > 0 {
		ue.Op = RandItem(eb.R, ops)
	} else {
		return eb.VarOrLit(t)
	}

	if eb.Deepen() {
		ue.X = eb.Expr(t)
	} else {
		ue.X = eb.VarOrLit(t)
	}

	return ue
}

func (eb *ExprBuilder) BinaryExpr(t Type) ast.Expr {
	ue := new(ast.BinaryExpr)

	ops := BinOps(t)
	if t.Name() == "bool" && eb.R.Intn(2) == 0 {
		// for booleans, we 50/50 between <bool> BOOL_OP <bool> and
		// <any comparable> COMPARISON <any comparable>.
		t = eb.pb.RandComparableType()
		ops = []token.Token{token.EQL, token.NEQ}
		if IsOrdered(t) {
			ops = append(ops, []token.Token{
				token.LSS, token.LEQ,
				token.GTR, token.GEQ}...)
		}
	}
	if len(ops) > 0 {
		ue.Op = RandItem(eb.R, ops)
	} else {
		return eb.VarOrLit(t)
	}

	t2 := t
	if ue.Op == token.SHR { // ensure rhs > 0 for shifts
		t2 = BT{"uint"}
	}

	// For some types, we need to ensure at least one leaf of the expr
	// tree is a variable, or we'll trigger compilation errors as
	// "constant overflows uint" on Exprs that end up being all
	// literals (and thus computable at compile time), and outside the
	// type's range.
	if _, isTP := t.(TypeParam); IsNumeric(t) || isTP {

		// LHS can be whatever
		if eb.Deepen() {
			ue.X = eb.Expr(t)
		} else {
			ue.X = eb.VarOrLit(t)
		}

		// Make sure the RHS is not a constant expression. The result
		// of len, min, and max are const when their args are consts,
		// so we need to avoid them.
		if vi, ok := eb.S.RandVarSubType(t2); ok && (vi.Name.Name != "len" && vi.Name.Name != "min" && vi.Name.Name != "max") {
			// If we can use some existing variable, do that.
			ue.Y = eb.SubTypeExpr(vi.Name, vi.Type, t2)
		} else {
			// Otherwise, cast from an int.
			vi, _ := eb.S.RandVar(BT{"int"})
			ue.Y = CastToType(t2, vi.Name)
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
			ue.Y = eb.VarOrLit(t2)
		}
	} else {
		ue.X = eb.VarOrLit(t)
		ue.Y = eb.VarOrLit(t2)
	}

	return ue
}

func (eb *ExprBuilder) Cast(t BasicType) *ast.CallExpr {

	// handle string([]byte) cast
	if t.Equal(BT{"string"}) {
		var arg ast.Expr
		if eb.Deepen() {
			arg = eb.Expr(SliceOf(BT{"byte"}))
		} else {
			arg = eb.VarOrLit(SliceOf(BT{"byte"}))
		}
		return &ast.CallExpr{
			Fun:  &ast.Ident{Name: t.N},
			Args: []ast.Expr{arg},
		}
	}

	// numeric casts
	t2 := t
	if IsNumeric(t) {
		t2 = eb.pb.RandNumericType()
		for strings.HasPrefix(t2.N, "float") {
			t2 = eb.pb.RandNumericType()
		}
	}

	return &ast.CallExpr{
		Fun:  &ast.Ident{Name: t.N},
		Args: []ast.Expr{eb.VarOrLit(t2)},
	}
}

// CallExpr returns a call expression with return value of type t. The
// function can be a builtin or stdlib function, a locally defined
// function variable, or a function literal that is immediately
// called.
func (eb *ExprBuilder) RandCallExpr(t Type) *ast.CallExpr {

	if eb.R.Intn(2) == 0 {
		if f, ok := eb.S.RandFuncRet(t); ok && !eb.C.inDefer {
			return eb.CallFunction(f, t)
		}
	} else {
		if v, m, ok := eb.S.RandMethod(t); ok {
			return eb.CallMethod(v, m, t)
		}
	}

	return eb.ConjureAndCallFunc(t)
}

func (eb *ExprBuilder) CallMethod(v Variable, m Method, ct ...Type) *ast.CallExpr {
	ce := &ast.CallExpr{}
	ce.Fun = &ast.SelectorExpr{
		X:   v.Name,
		Sel: m.Name,
	}

	for _, arg := range m.Func.Args {
		if eb.Deepen() {
			ce.Args = append(ce.Args, eb.Expr(arg))
		} else {
			ce.Args = append(ce.Args, eb.VarOrLit(arg))
		}
	}

	return ce
}

// CallFunction builds an ast.CallExpr calling the given function,
// taking care of setting up its arguments, including for special
// builtins like copy() or functions that require custom handling.
func (eb *ExprBuilder) CallFunction(f FuncType, ct ...Type) *ast.CallExpr {

	if f.N == "" {
		panic("CallFunction on unnamed FunctType")
	}

	ce := &ast.CallExpr{}

	if f.Pkg != "" {
		ce.Fun = &ast.SelectorExpr{
			X:   &ast.Ident{Name: f.Pkg},
			Sel: &ast.Ident{Name: f.N},
		}
	} else {
		ce.Fun = &ast.Ident{Name: f.N}
	}

	switch f.N {

	case "copy":
		var t1, t2 Type
		if eb.R.Intn(3) == 0 {
			t1, t2 = SliceOf(BT{N: "byte"}), BT{N: "string"}
		} else {
			t1 = SliceOf(eb.pb.RandType())
			t2 = t1
		}
		if eb.Deepen() {
			ce.Args = []ast.Expr{eb.Expr(t1), eb.Expr(t2)}
		} else {
			ce.Args = []ast.Expr{eb.VarOrLit(t1), eb.VarOrLit(t2)}
		}

	case "len":
		var t Type
		if eb.R.Intn(3) < 2 {
			t = SliceOf(eb.pb.RandType())
		} else {
			t = BT{"string"}
		}
		if eb.Deepen() {
			ce.Args = []ast.Expr{eb.Expr(t)}
		} else {
			ce.Args = []ast.Expr{eb.VarOrLit(t)}
		}

	case "min", "max":
		if len(ct) == 0 {
			panic("min/max need additional type arg")
		}
		t := ct[0]

		for i := 0; i < 1+eb.R.Intn(5); i++ {
			if eb.Deepen() {
				ce.Args = append(ce.Args, eb.Expr(t))
			} else {
				ce.Args = append(ce.Args, eb.VarOrLit(t))
			}
		}

	case "Offsetof":
		var sl *ast.SelectorExpr

		// If we can get a variable, use that (as long it has at least
		// one field). Otherwise, conjure a literal of a random struct
		// type.
		v, ok := eb.S.RandStruct()
		if ok && len(v.Type.(StructType).Fnames) > 0 {
			sl = &ast.SelectorExpr{
				X:   v.Name,
				Sel: &ast.Ident{Name: RandItem(eb.R, v.Type.(StructType).Fnames)},
			}
		} else {
			var st StructType
			for len(st.Fnames) == 0 {
				st = eb.pb.RandStructType()
			}
			sl = &ast.SelectorExpr{
				X:   eb.VarOrLit(st),
				Sel: &ast.Ident{Name: RandItem(eb.R, st.Fnames)},
			}
		}
		ce.Args = []ast.Expr{sl}

	case "Sizeof", "Alignof":
		t := eb.pb.RandType()
		_, isPtr := t.(PointerType)
		_, isFnc := t.(FuncType)
		_, isInt := t.(InterfaceType)
		for isPtr || isFnc || isInt {
			t = eb.pb.RandType()
			_, isPtr = t.(PointerType)
			_, isFnc = t.(FuncType)
			_, isInt = t.(InterfaceType)
		}
		if eb.Deepen() {
			ce.Args = []ast.Expr{eb.Expr(t)}
		} else {
			ce.Args = []ast.Expr{eb.VarOrLit(t)}
		}

	case "SliceData":
		if len(ct) == 0 {
			panic("unsafe.SliceData needs additional type arg")
		}
		t := SliceOf(ct[0].(PointerType).Base())
		if eb.Deepen() {
			ce.Args = []ast.Expr{eb.Expr(t)}
		} else {
			ce.Args = []ast.Expr{eb.VarOrLit(t)}
		}

	case "DeepEqual":
		t1, t2 := eb.pb.RandType(), eb.pb.RandType()
		if eb.Deepen() {
			ce.Args = []ast.Expr{eb.Expr(t1), eb.Expr(t2)}
		} else {
			ce.Args = []ast.Expr{eb.VarOrLit(t1), eb.VarOrLit(t2)}
		}

	case "All":
		if len(ct) == 0 {
			panic("slices.All needs additional type arg")
		}
		t := SliceOf(ct[0])
		if eb.Deepen() {
			ce.Args = []ast.Expr{eb.Expr(t)}
		} else {
			ce.Args = []ast.Expr{eb.VarOrLit(t)}
		}

	case "Print":
		for range 1 + eb.R.Intn(4) {
			t := eb.pb.RandType()
			if eb.R.Intn(3) < 2 {
				if v, ok := eb.S.RandAssignable(); ok {
					ce.Args = append(ce.Args, v.Name)
				} else {
					ce.Args = append(ce.Args, eb.VarOrLit(t))
				}
			} else {
				ce.Args = append(ce.Args, eb.VarOrLit(t))
			}
		}
	default:
		if f.Args == nil || f.Ret == nil {
			panic("CallFunction: missing special handling for " + f.N)
		}
		args := make([]ast.Expr, 0, len(f.Args))
		for _, arg := range f.Args {
			if ep, ok := arg.(EllipsisType); ok {
				arg = ep.Base
			}
			if eb.Deepen() && f.Local {
				// Cannot call Expr with casts, because UnaryExpr
				// could return e.g. -11 which cannot be cast to uint.
				args = append(args, eb.Expr(arg))
			} else {
				args = append(args, eb.VarOrLit(arg))
			}
		}
		ce.Args = args
	}

	return ce
}

// CallFunctionVar builds an ast.CallExpr calling the local function
// in variable v
func (eb *ExprBuilder) CallFunctionVar(v Variable, ct ...Type) *ast.CallExpr {
	f, ok := v.Type.(FuncType)
	if !ok {
		panic("CallFunctionVar: not a function: " + v.Name.Name)
	}

	if f.Pkg != "" {
		panic("CallFunctionVar: called with non-local function " + v.Name.Name)
	}

	ce := &ast.CallExpr{}
	ce.Fun = v.Name

	args := make([]ast.Expr, 0, len(f.Args))
	for _, arg := range f.Args {
		if ep, ok := arg.(EllipsisType); ok {
			arg = ep.Base
		}
		if eb.Deepen() && f.Local {
			// Cannot call Expr with casts, because UnaryExpr
			// could return e.g. -11 which cannot be cast to uint.
			args = append(args, eb.Expr(arg))
		} else {
			args = append(args, eb.VarOrLit(arg))
		}
	}
	ce.Args = args

	return ce

}

func (eb *ExprBuilder) MakeAppendCall(t SliceType) *ast.CallExpr {
	ce := &ast.CallExpr{Fun: AppendIdent}

	t2 := t.Base()
	ellips := token.Pos(0)
	if eb.R.Intn(3) == 0 { // 2nd arg is ...
		t2 = t
		ellips = token.Pos(1)
	}

	if eb.Deepen() {
		ce.Args = []ast.Expr{eb.Expr(t), eb.Expr(t2)}
	} else {
		ce.Args = []ast.Expr{eb.VarOrLit(t), eb.VarOrLit(t2)}
	}
	ce.Ellipsis = ellips

	return ce
}

func (eb *ExprBuilder) MakeMakeCall(t Type) *ast.CallExpr {

	ce := &ast.CallExpr{Fun: MakeIdent}

	switch t := t.(type) {
	case SliceType:
		tn := t.Base().Ast()
		if eb.Deepen() {
			ce.Args = []ast.Expr{&ast.ArrayType{Elt: tn}, eb.BinaryExpr(BT{"int"})}
		} else {
			ce.Args = []ast.Expr{&ast.ArrayType{Elt: tn}, eb.VarOrLit(BT{"int"})}
		}
	case MapType:
		tk, tv := t.KeyT.Ast(), t.ValueT.Ast()
		if eb.Deepen() {
			ce.Args = []ast.Expr{&ast.MapType{Key: tk, Value: tv}, eb.BinaryExpr(BT{"int"})}
		} else {
			ce.Args = []ast.Expr{&ast.MapType{Key: tk, Value: tv}, eb.VarOrLit(BT{"int"})}
		}
	default:
		panic("MakeMakeCall: invalid type " + t.Name())
	}

	return ce
}

func (eb *ExprBuilder) ConjureAndCallFunc(t Type) *ast.CallExpr {

	ft := &FuncType{Args: []Type{}, Ret: []Type{t}, Local: true}
	for i := 0; i < eb.R.Intn(5); i++ {
		ft.Args = append(ft.Args, eb.pb.RandType())
	}

	var retExpr ast.Expr
	if eb.Deepen() {
		retExpr = eb.Expr(t)
	} else {
		retExpr = eb.VarOrLit(t)
	}

	p, r := ft.MakeFieldLists(false, 0)
	fl := &ast.FuncLit{
		Type: &ast.FuncType{Params: p, Results: r},
		Body: &ast.BlockStmt{List: []ast.Stmt{
			eb.pb.sb.AssignStmt(),                         // one statement
			&ast.ReturnStmt{Results: []ast.Expr{retExpr}}, // the return
		}},
	}

	// If we are in a defer, optionally add a recover call before
	// the return statement.
	if eb.C.inDefer && eb.R.Intn(4) == 0 {
		fl.Body.List = append(
			[]ast.Stmt{&ast.ExprStmt{&ast.CallExpr{Fun: &ast.Ident{Name: "recover"}}}},
			fl.Body.List...,
		)
	}

	// Finally, call it.
	args := make([]ast.Expr, 0, len(ft.Args))
	for _, arg := range ft.Args {
		args = append(args, eb.VarOrLit(arg))
	}
	return &ast.CallExpr{Fun: fl, Args: args}
}
