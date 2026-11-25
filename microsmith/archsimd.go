//go:build goexperiment.simd

package microsmith

import (
	"reflect"
	"simd/archsimd"
	"slices"
)

var SimdTypes []ExternalType

// var SimdInt32x8 = ExternalType{
// 	Pkg:     "archsimd",
// 	N:       "Int32x8",
// 	Builder: nil,
// }

// var SimdMask32x8 = ExternalType{
// 	Pkg:     "archsimd",
// 	N:       "Mask32x8",
// 	Builder: nil,
// }

// var SimdFloat32x8 = ExternalType{
// 	Pkg:     "archsimd",
// 	N:       "Float32x8",
// 	Builder: nil,
// }

// var SimdInt64x8 = ExternalType{
// 	Pkg:     "archsimd",
// 	N:       "Int64x8",
// 	Builder: nil,
// }

// var SimdMask64x8 = ExternalType{
// 	Pkg:     "archsimd",
// 	N:       "Mask64x8",
// 	Builder: nil,
// }

// var SimdFloat64x8 = ExternalType{
// 	Pkg:     "archsimd",
// 	N:       "Float64x8",
// 	Builder: nil,
// }

func init() {
	ImportedPkgs = append(ImportedPkgs, "simd/archsimd")

	stypes := []reflect.Type{
		reflect.TypeFor[archsimd.Float32x16](),
		reflect.TypeFor[archsimd.Float32x4](),
		reflect.TypeFor[archsimd.Float32x8](),
		reflect.TypeFor[archsimd.Float64x2](),
		reflect.TypeFor[archsimd.Float64x4](),
		reflect.TypeFor[archsimd.Float64x8](),
		reflect.TypeFor[archsimd.Int16x16](),
		reflect.TypeFor[archsimd.Int16x32](),
		reflect.TypeFor[archsimd.Int16x8](),
		reflect.TypeFor[archsimd.Int32x16](),
		reflect.TypeFor[archsimd.Int32x4](),
		reflect.TypeFor[archsimd.Int32x8](),
		reflect.TypeFor[archsimd.Int64x2](),
		reflect.TypeFor[archsimd.Int64x4](),
		reflect.TypeFor[archsimd.Int64x8](),
		reflect.TypeFor[archsimd.Int8x16](),
		reflect.TypeFor[archsimd.Int8x32](),
		reflect.TypeFor[archsimd.Int8x64](),
		reflect.TypeFor[archsimd.Mask16x8](),
		reflect.TypeFor[archsimd.Mask16x16](),
		reflect.TypeFor[archsimd.Mask16x32](),
		reflect.TypeFor[archsimd.Mask32x16](),
		reflect.TypeFor[archsimd.Mask32x4](),
		reflect.TypeFor[archsimd.Mask32x8](),
		reflect.TypeFor[archsimd.Mask64x2](),
		reflect.TypeFor[archsimd.Mask64x4](),
		reflect.TypeFor[archsimd.Mask64x8](),
		reflect.TypeFor[archsimd.Mask8x16](),
		reflect.TypeFor[archsimd.Mask8x32](),
		reflect.TypeFor[archsimd.Mask8x64](),
		reflect.TypeFor[archsimd.Uint16x16](),
		reflect.TypeFor[archsimd.Uint16x32](),
		reflect.TypeFor[archsimd.Uint16x8](),
		reflect.TypeFor[archsimd.Uint32x16](),
		reflect.TypeFor[archsimd.Uint32x4](),
		reflect.TypeFor[archsimd.Uint32x8](),
		reflect.TypeFor[archsimd.Uint64x2](),
		reflect.TypeFor[archsimd.Uint64x4](),
		reflect.TypeFor[archsimd.Uint64x8](),
		reflect.TypeFor[archsimd.Uint8x16](),
		reflect.TypeFor[archsimd.Uint8x32](),
		reflect.TypeFor[archsimd.Uint8x64](),
	}

	for _, t := range stypes {
		et := ExternalType{
			Pkg:     "archsimd",
			N:       t.Name(),
			Builder: nil,
		}
		SimdTypes = append(SimdTypes, et)
	}

	for i := range len(stypes) {
		SimdTypes[i].Methods = MakeMethods(stypes[i])
	}

	for _, et := range SimdTypes {
		StdTypes = append(StdTypes, et)
	}
}

func NameToType(typ reflect.Type) (Type, bool) {

	// basic types
	if bt, ok := FindBaseType(typ.Name()); ok {
		return bt, true
	}

	// composite types
	switch typ.Kind() {
	case reflect.Slice:
		bt, ok := FindBaseType(typ.Elem().Name())
		if !ok {
			panic("what")
		}
		return SliceOf(bt), true
	case reflect.Pointer:
		baseType := typ.Elem()
		if baseType.Kind() == reflect.Array {
			bt, ok := FindBaseType(baseType.Elem().Name())
			if !ok {
				panic("what")
			}
			return PointerOf(ArrayOf(bt, 16)), true
		}
	}

	// external types
	i := slices.IndexFunc(SimdTypes, func(et ExternalType) bool {
		return et.N == typ.Name()
	})
	if i >= 0 {
		return SimdTypes[i], true
	}

	return BT{}, false
}

func FindBaseType(str string) (Type, bool) {
	i := slices.IndexFunc(BaseTypes, func(t Type) bool {
		bt, ok := t.(BT)
		return ok && bt.N == str
	})
	if i >= 0 {
		return BaseTypes[i], true
	}
	return BT{}, false
}
