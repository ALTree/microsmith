package microsmith

import (
	"math/rand"
	"strconv"
)

// Context holds all the contextual information needed while
// generating a random program.
type Context struct {

	// program-wide settings for the fuzzer
	programConf ProgramConf

	// all the Costraints available declared in the program
	constraints []Constraint

	// package-wide scope of vars and func available to the code in a
	// given moment
	scope *Scope

	// Function-level scope of the type parameters available to the
	// code in the body. Reset and re-filled in when declaring a new
	// function.
	typeparams *Scope
}

func NewContext(pc ProgramConf) *Context {
	return &Context{
		programConf: pc,
	}
}

// ProgramConf holds program-wide configuration settings that change
// the kind of programs that are generated.
type ProgramConf struct {
	MultiPkg   bool // for -multipkg
	TypeParams bool // for -tp
}

// --------------------------------
//   Types Randomizers
// --------------------------------

// Returns a slice of n random types, including composite types
// (structs, array, maps, chans).
func (pb ProgramBuilder) RandTypes(n int) []Type {
	types := make([]Type, n)
	for i := 0; i < n; i++ {
		types[i] = pb.RandType()
	}
	return types
}

// Returns a single random type (including structs, array, maps,
// chans).
func (pb ProgramBuilder) RandType() Type {
	switch pb.rs.Intn(15) {
	case 0, 1:
		return ArrayOf(pb.RandType())
	case 2:
		return ChanOf(pb.RandType())
	case 3, 4:
		return MapOf(
			pb.RandAddressableType(),
			pb.RandType(),
		)
	case 5, 6:
		return PointerOf(pb.RandType())
	case 7, 8:
		return pb.RandStructType()
	case 9:
		return pb.RandFuncType()
	default:
		return pb.RandBaseType()
	}
}

func (pb ProgramBuilder) RandAddressableType() Type {
	types := make([]Type, 0, 32)

	// collect addressable Base Types
	for _, t := range AllTypes {
		if t.Addressable() {
			types = append(types, t)
		}
	}

	// look for addressable type parameters
	if tp := pb.ctx.typeparams; tp != nil {
		for _, v := range *tp {
			if v.Type.Addressable() {
				types = append(types, MakeTypeParam(v))
			}
		}
	}
	return types[pb.rs.Intn(len(types))]
}

// Returns a single BaseType (primitives, or a type parameter).
func (pb ProgramBuilder) RandBaseType() Type {
	if tp := pb.ctx.typeparams; tp != nil {
		i := pb.rs.Intn(len(AllTypes) + len(*tp))
		if i < len(AllTypes) {
			return AllTypes[i]
		} else {
			return MakeTypeParam((*tp)[i-len(AllTypes)])
		}
	} else {
		return AllTypes[rand.Intn(len(AllTypes))]
	}
}

func (pb ProgramBuilder) RandStructType() StructType {
	st := StructType{"ST", []Type{}, []string{}}
	for i := 0; i < pb.rs.Intn(6); i++ {
		t := pb.RandType()
		st.Ftypes = append(st.Ftypes, t)
		st.Fnames = append(st.Fnames, Ident(t)+strconv.Itoa(i))
	}
	return st
}

func (pb ProgramBuilder) RandFuncType() FuncType {
	args := make([]Type, 0, pb.rs.Intn(5))

	// arguments
	for i := 0; i < cap(args); i++ {
		args = append(args, pb.RandType())
	}

	// 0.25 of making the last parameter variadic
	if len(args) > 0 && pb.rs.Intn(4) == 0 {
		args[len(args)-1] = EllipsisType{Base: args[len(args)-1]}
	}

	// return type
	ret := []Type{pb.RandType()}

	return FuncType{"FU", args, ret, true}
}
