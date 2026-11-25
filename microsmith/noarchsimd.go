//go:build !goexperiment.simd

package microsmith

import (
	"reflect"
	"slices"
)

func NameToType(typ reflect.Type) (Type, bool) {
	name := typ.Name()
	i := slices.IndexFunc(BaseTypes, func(t Type) bool {
		bt, ok := t.(BT)
		return ok && bt.N == name
	})
	if i >= 0 {
		return BaseTypes[i], true
	}
	return BT{}, false
}
