//go:build goexperiment.simd

package microsmith

import (
	"simd/archsimd"
	"slices"
)

var SimdInt32x8 = ExternalType{
	Pkg:     "archsimd",
	N:       "Int32x8",
	Builder: nil,
}

var SimdMask32x8 = ExternalType{
	Pkg:     "archsimd",
	N:       "Mask32x8",
	Builder: nil,
}

var SimdFloat32x8 = ExternalType{
	Pkg:     "archsimd",
	N:       "Float32x8",
	Builder: nil,
}

func init() {
	ImportedPkgs = append(ImportedPkgs, "simd/archsimd")

	SimdInt32x8.Methods = MakeMethods[archsimd.Int32x8]()
	StdTypes = append(StdTypes, SimdInt32x8)
	SimdMask32x8.Methods = MakeMethods[archsimd.Mask32x8]()
	StdTypes = append(StdTypes, SimdMask32x8)
	SimdFloat32x8.Methods = MakeMethods[archsimd.Float32x8]()
	StdTypes = append(StdTypes, SimdFloat32x8)
}

func NameToType(name string) (Type, bool) {
	i := slices.IndexFunc(BaseTypes, func(t Type) bool {
		bt, ok := t.(BT)
		return ok && bt.N == name
	})
	if i >= 0 {
		return BaseTypes[i], true
	}

	switch name {
	case "Int32x8":
		return SimdInt32x8, true
	case "Mask32x8":
		return SimdMask32x8, true
	case "Float32x8":
		return SimdFloat32x8, true
	default:
		return BT{}, false
	}
}
