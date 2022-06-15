package microsmith

import (
	"go/ast"
	"go/token"
	"strconv"
	"strings"
)

// --------------------------------
//  ExprBuilder Type
// --------------------------------

type ExprBuilder struct {
	pb    *PackageBuilder
	depth int // how deep the expr hierarchy is
}

func NewExprBuilder(pb *PackageBuilder) *ExprBuilder {
	return &ExprBuilder{
		pb: pb,
	}
}

// --------------------------------
//  Accessors
// --------------------------------

func (eb ExprBuilder) S() *Scope {
	return eb.pb.ctx.scope
}

// --------------------------------
//  Builder Methods
// --------------------------------

// Returns true if the expression tree currently being built is
// allowed to become deeper.
func (eb *ExprBuilder) Deepen() bool {
	return (eb.depth <= 6) && (eb.pb.rs.Float64() < 0.7)
}

func (eb *ExprBuilder) BasicLit(t BasicType) ast.Expr {
	bl := new(ast.BasicLit)
	switch t.Name() {
	case "byte", "uint", "uintptr", "int", "int8", "int16", "int32", "int64", "any":
		bl.Kind = token.INT
		bl.Value = strconv.Itoa(eb.pb.rs.Intn(100))
	case "rune":
		bl.Kind = token.CHAR
		bl.Value = RandRune()
	case "float32", "float64":
		bl.Kind = token.FLOAT
		bl.Value = strconv.FormatFloat(999*(eb.pb.rs.Float64()), 'f', 1, 64)
	case "complex128":
		// There's no complex basiclit, generate an IMAG
		bl.Kind = token.IMAG
		bl.Value = strconv.FormatFloat(99*(eb.pb.rs.Float64()), 'f', 2, 64) + "i"
	case "bool":
		if eb.pb.rs.Intn(2) == 0 {
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
		if eb.pb.rs.Intn(4) > 0 { // plain array literal
			for i := 0; i < eb.pb.rs.Intn(5); i++ {
				if eb.Deepen() {
					elems = append(elems, eb.Expr(t.Base()))
				} else {
					elems = append(elems, eb.VarOrLit(t.Base()))
				}
			}
		} else { // keyed literal (a single one, since dups are a compile error)
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
	case ChanType:
		panic("No CompositeLit of type chan")
	case MapType:
		cl := &ast.CompositeLit{Type: t.Ast()}
		var e *ast.KeyValueExpr
		if eb.Deepen() {
			e = &ast.KeyValueExpr{
				Key:   eb.Expr(t.KeyT),
				Value: eb.Expr(t.ValueT),
			}
		} else {
			e = &ast.KeyValueExpr{
				Key:   eb.VarOrLit(t.KeyT),
				Value: eb.VarOrLit(t.ValueT),
			}
		}
		// Duplicate map keys are a compile error, but avoiding them
		// is hard, so only have 1 element for now.
		cl.Elts = []ast.Expr{e}
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

	switch t := t.(type) {

	case BasicType, TypeParam:
		switch eb.pb.rs.Intn(10) {
		case 0, 1:
			return eb.UnaryExpr(t)
		case 2, 3:
			return eb.UnaryExpr(t)
		case 4, 5, 6, 7:
			return eb.BinaryExpr(t)
		default:
			return eb.RandCallExpr(t, NOTDEFER)
		}

	case ArrayType:
		switch eb.pb.rs.Intn(4) {
		case 0:
			return eb.MakeAppendCall(t)
		default:
			return eb.VarOrLit(t)
		}

	case ChanType, FuncType, MapType, StructType:
		return eb.VarOrLit(t)

	case PointerType:
		// Either return a literal of the requested pointer type, &x
		// with x of type t.Base(), or nil.
		vt, typeInScope := eb.S().RandVar(t)
		vst, baseInScope := eb.S().RandVar(t.Base())
		if typeInScope && baseInScope {
			if eb.pb.rs.Intn(2) == 0 {
				return vt.Name
			} else {
				return &ast.UnaryExpr{
					Op: token.AND,
					X:  vst.Name,
				}
			}
		} else if typeInScope {
			return vt.Name
		} else if baseInScope {
			return &ast.UnaryExpr{
				Op: token.AND,
				X:  vst.Name,
			}
		} else {
			// TODO(alb): this is not correct because Expr's contract
			// says it returns an ast.Expr of type t, but here we may
			// return a non-typed nil. This nil is fine in
			//
			//   var p *int
			//   p = nil
			//
			// but it cannot be used as a general type t expr, for
			// example this doesn't compile:
			//
			//   var i int
			//   i = *nil
			//
			// That nil was returned by Expr() when requested an *int
			// expr, but it's actually untyped.
			//
			// For now this works because we only dereference a
			// pointer returned by Expr() in UnaryExpr(), and there we
			// only do that when there's a pointer of that type in
			// scope, so above we'll always enter the if typeInScope.
			return &ast.Ident{Name: "nil"}
		}

	default:
		panic("Unimplemented type " + t.Name())
	}
}

func (eb *ExprBuilder) VarOrLit(t Type) ast.Expr {

	// If t is a type parameter, 50/50 between returning a variable
	// and building a literal; except for type parameters that don't
	// allow literals (like interface { int | []int }); for those it's
	// always a variable.
	if tp, ok := t.(TypeParam); ok {
		if tp.HasLiterals() && eb.pb.rs.Intn(2) == 0 {
			return eb.TypeParamLit(tp)
		}
		if v, ok := eb.S().RandVar(t); !ok {
			panic("VarOrLit couldn't find a type parameter variable")
		} else {
			return v.Name
		}
	}

	vst, typeCanDerive := eb.S().RandVarSubType(t)

	if !typeCanDerive || !eb.Deepen() {
		switch t := t.(type) {
		case BasicType:
			bl := eb.BasicLit(t)
			if t.NeedsCast() {
				bl = &ast.CallExpr{Fun: t.Ast(), Args: []ast.Expr{bl}}
			}
			return bl
		case ArrayType, MapType:
			if eb.pb.rs.Intn(3) == 0 {
				return eb.MakeMakeCall(t)
			} else {
				return eb.CompositeLit(t)
			}
		case StructType:
			return eb.CompositeLit(t)
		case ChanType:
			// No literal of type Chan, but we can return make(chan t)
			return &ast.CallExpr{
				Fun: &ast.Ident{Name: "make"},
				Args: []ast.Expr{
					&ast.ChanType{Dir: 3, Value: t.Base().Ast()},
				},
			}
		case PointerType, FuncType:
			return &ast.Ident{Name: "nil"}
		case TypeParam:
			return eb.TypeParamLit(t)

		default:
			panic("unhandled type " + t.Name())
		}
	}

	// Slice, once in a while.
	if _, ok := vst.Type.(ArrayType); ok {
		if t.Equal(vst.Type) && eb.pb.rs.Intn(4) == 0 {
			return eb.SliceExpr(vst)
		}
	}
	if bt, ok := vst.Type.(BasicType); ok && bt.N == "string" {
		if t.Equal(vst.Type) && eb.pb.rs.Intn(4) == 0 {
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
		if eb.pb.rs.Intn(2) == 0 && eb.Deepen() {
			args = append(args, eb.Expr(t))
		} else {
			args = append(args, eb.VarOrLit(t))
		}
	}
	return &ast.CallExpr{Fun: e, Args: args}
}

// Returns e[...]
func (eb *ExprBuilder) IndexExpr(e ast.Expr) *ast.IndexExpr {
	var i ast.Expr
	if eb.pb.rs.Intn(2) == 0 && eb.Deepen() {
		i = eb.BinaryExpr(BT{"int"})
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
	indV, hasInt := eb.S().RandVar(BT{"int"})
	if hasInt && eb.Deepen() {
		if eb.pb.rs.Intn(8) > 0 {
			low = &ast.BinaryExpr{
				X:  indV.Name,
				Op: token.ADD,
				Y:  eb.Expr(BT{"int"}),
			}
		}
		if eb.pb.rs.Intn(8) > 0 {
			high = &ast.BinaryExpr{
				X:  eb.Expr(BT{"int"}),
				Op: token.ADD,
				Y:  indV.Name,
			}
		}
	} else {
		if eb.pb.rs.Intn(8) > 0 {
			low = &ast.BasicLit{
				Kind:  token.INT,
				Value: strconv.Itoa(eb.pb.rs.Intn(8)),
			}
		}
		if eb.pb.rs.Intn(8) > 0 {
			high = &ast.BasicLit{
				Kind:  token.INT,
				Value: strconv.Itoa(8 + eb.pb.rs.Intn(17)),
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
	if eb.pb.rs.Intn(2) == 0 && eb.S().Has(PointerOf(t)) {
		ue.Op = token.MUL
		// See comment in Expr() for PointerType on why we must call
		// Expr() here.
		ue.X = eb.Expr(PointerOf(t))
		return ue
	}

	if ops := UnaryOps(t); len(ops) > 0 {
		ue.Op = RandItem(eb.pb.rs, ops)
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
	if t.Name() == "bool" && eb.pb.rs.Intn(2) == 0 {
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
		ue.Op = RandItem(eb.pb.rs, ops)
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

		// make sure the RHS is not a constant expression
		if vi, ok := eb.S().RandVarSubType(t2); ok {
			ue.Y = eb.SubTypeExpr(vi.Name, vi.Type, t2)
		} else { // otherwise, cast from an int
			vi, ok := eb.S().RandVar(BT{"int"})
			if !ok {
				panic("BinaryExpr: no int in scope")
			}
			ue.Y = &ast.CallExpr{
				Fun:  TypeIdent(t2.Name()),
				Args: []ast.Expr{vi.Name},
			}
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

type CallExprType int

const (
	DEFER    CallExprType = 0
	NOTDEFER CallExprType = 1
)

// CallExpr returns a call expression involving a random function with
// return value of type t.
func (eb *ExprBuilder) RandCallExpr(t Type, cet CallExprType) *ast.CallExpr {
	if v, ok := eb.S().RandFuncRet(t); ok && (cet == NOTDEFER || v.Type.(FuncType).Local) {
		name := v.Name.Name
		switch {
		case name == "copy":
			return eb.MakeCopyCall()
		case name == "len":
			return eb.MakeLenCall()
		case strings.HasPrefix(name, "unsafe."):
			return eb.MakeUnsafeCall(v)
		case strings.HasPrefix(name, "reflect."):
			return eb.MakeReflectCall(v)
		default:
			return eb.MakeFuncCall(v)
		}
	} else {
		// No functions returning t in scope. Conjure a random one,
		// and call it.

		// Random func type, 2 parameters, return type t.
		ft := &FuncType{
			"FU",
			[]Type{eb.pb.RandType(), eb.pb.RandType()},
			[]Type{t},
			true,
		}

		// Empty body (avoid excessive nesting, we are just building
		// an expression after all), except for the return statement.
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
				eb.pb.sb.AssignStmt(),                         // one Stmt
				&ast.ReturnStmt{Results: []ast.Expr{retExpr}}, // the return
			}},
		}

		// if we are in a defer, optionally add a recover call before
		// the return statement.
		if cet == DEFER && eb.pb.rs.Intn(4) == 0 {
			fl.Body.List = append(
				[]ast.Stmt{&ast.ExprStmt{&ast.CallExpr{Fun: &ast.Ident{Name: "recover"}}}},
				fl.Body.List...,
			)
		}

		// and then call it
		args := make([]ast.Expr, 0, len(ft.Args))
		for _, arg := range ft.Args {
			args = append(args, eb.VarOrLit(arg))
		}
		return &ast.CallExpr{Fun: fl, Args: args}
	}
}

func (eb *ExprBuilder) MakeFuncCall(v Variable) *ast.CallExpr {
	if fnc, ok := v.Type.(FuncType); ok {
		args := make([]ast.Expr, 0, len(fnc.Args))
		for _, arg := range fnc.Args {
			arg := arg
			if ep, ok := arg.(EllipsisType); ok {
				arg = ep.Base
			}
			if eb.Deepen() && fnc.Local {
				// Cannot call Expr with casts, because Expr could
				// return UnaryExpr(Literal) like -11 which cannot
				// be cast to e.g. int.
				args = append(args, eb.Expr(arg))
			} else {
				args = append(args, eb.VarOrLit(arg))
			}
		}
		return &ast.CallExpr{
			Fun:  &ast.Ident{Name: v.Name.Name},
			Args: args,
		}
	}

	panic("unreachable")
}

func (eb *ExprBuilder) MakeAppendCall(t ArrayType) *ast.CallExpr {
	ce := &ast.CallExpr{Fun: AppendIdent}

	t2 := t.Base()
	ellips := token.Pos(0)
	if eb.pb.rs.Intn(3) == 0 { // 2nd arg is ...
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

func (eb *ExprBuilder) MakeCopyCall() *ast.CallExpr {
	var typ1, typ2 Type
	if eb.pb.rs.Intn(3) == 0 {
		typ1, typ2 = ArrayOf(BT{N: "byte"}), BT{N: "string"}
	} else {
		typ1 = ArrayOf(eb.pb.RandBaseType())
		typ2 = typ1
	}

	ce := &ast.CallExpr{Fun: CopyIdent}
	if eb.Deepen() {
		ce.Args = []ast.Expr{eb.Expr(typ1), eb.Expr(typ2)}
	} else {
		ce.Args = []ast.Expr{eb.VarOrLit(typ1), eb.VarOrLit(typ2)}
	}

	return ce
}

func (eb *ExprBuilder) MakeLenCall() *ast.CallExpr {
	var typ Type
	if eb.pb.rs.Intn(2) == 0 {
		typ = ArrayOf(eb.pb.RandBaseType())
	} else {
		typ = BT{"string"}
	}
	ce := &ast.CallExpr{Fun: LenIdent}
	if eb.Deepen() {
		ce.Args = []ast.Expr{eb.Expr(typ)}
	} else {
		ce.Args = []ast.Expr{eb.VarOrLit(typ)}
	}

	return ce
}

func (eb *ExprBuilder) MakeMakeCall(t Type) *ast.CallExpr {

	ce := &ast.CallExpr{Fun: MakeIdent}

	switch t := t.(type) {
	case ArrayType:
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

func (eb *ExprBuilder) MakeUnsafeCall(fun Variable) *ast.CallExpr {
	fName := fun.Name.Name[len("unsafe."):]
	ce := &ast.CallExpr{
		Fun: &ast.SelectorExpr{
			X:   &ast.Ident{Name: "unsafe"},
			Sel: &ast.Ident{Name: fName},
		},
	}

	// Offsetof needs a struct selector expr of the form:
	//
	//   X.F
	//
	// where X is either a struct literal or a variable of struct
	// type.
	if fName == "Offsetof" {
		var sl *ast.SelectorExpr

		// if we can get a variable, use that (as long it has at least
		// one field). Otherwise, conjure a literal of a random struct
		// type.
		v, ok := eb.S().RandStruct()
		if ok && len(v.Type.(StructType).Fnames) > 0 {
			sl = &ast.SelectorExpr{
				X:   v.Name,
				Sel: &ast.Ident{Name: RandItem(eb.pb.rs, v.Type.(StructType).Fnames)},
			}
		} else {
			var st StructType
			for len(st.Fnames) == 0 {
				st = eb.pb.RandStructType()
			}
			sl = &ast.SelectorExpr{
				X:   eb.VarOrLit(st),
				Sel: &ast.Ident{Name: RandItem(eb.pb.rs, st.Fnames)},
			}
		}

		ce.Args = []ast.Expr{sl}
		return ce
	}

	typ := eb.pb.RandBaseType()
	if eb.Deepen() {
		ce.Args = []ast.Expr{eb.Expr(typ)}
	} else {
		ce.Args = []ast.Expr{eb.VarOrLit(typ)}
	}

	return ce
}

func (eb *ExprBuilder) MakeReflectCall(fun Variable) *ast.CallExpr {
	fName := fun.Name.Name[len("reflect."):]
	ce := &ast.CallExpr{
		Fun: &ast.SelectorExpr{
			X:   &ast.Ident{Name: "reflect"},
			Sel: &ast.Ident{Name: fName},
		},
	}

	switch fName {
	case "DeepEqual":
		t1, t2 := eb.pb.RandType(), eb.pb.RandType()
		if eb.Deepen() {
			ce.Args = []ast.Expr{eb.Expr(t1), eb.Expr(t2)}
		} else {
			ce.Args = []ast.Expr{eb.VarOrLit(t1), eb.VarOrLit(t2)}
		}
	default:
		panic("not implemented")
	}

	return ce
}
