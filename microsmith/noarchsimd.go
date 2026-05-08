//go:build !goexperiment.simd

package microsmith

import "slices"

func NameToType(name string) (Type, bool) {
	i := slices.IndexFunc(BaseTypes, func(t Type) bool {
		bt, ok := t.(BT)
		return ok && bt.N == name
	})
	if i >= 0 {
		return BaseTypes[i], true
	}
	return BT{}, false
}
