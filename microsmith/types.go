package microsmith

type Type int

const (
	NoType = iota
	TypeInt
	TypeBool
	TypeString

	TypeIntArr
	TypeBoolArr
	TypeStringArr
)

func (t Type) IsBasic() bool {
	switch t {
	case TypeInt, TypeBool, TypeString:
		return true
	default:
		return false
	}
}

// given a type, it returns the corresponding array type
func (t Type) Arr() Type {
	if !t.IsBasic() {
		panic("Arr: non-basic type " + t.String())
	}
	switch t {
	case TypeInt:
		return TypeIntArr
	case TypeBool:
		return TypeBoolArr
	case TypeString:
		return TypeStringArr
	default:
		panic("Arr: unimplemented for type " + t.String())
	}
}

// given an array type, it returns the corresponding base type
func (t Type) Base() Type {
	if t.IsBasic() {
		panic("Arr: basic type " + t.String())
	}
	switch t {
	case TypeIntArr:
		return TypeInt
	case TypeBoolArr:
		return TypeBool
	case TypeStringArr:
		return TypeString
	default:
		panic("Arr: unimplemented for type " + t.String())
	}
}

func (t Type) VarName() string {
	switch t {
	case TypeInt:
		return "I"
	case TypeBool:
		return "B"
	case TypeString:
		return "S"
	case TypeIntArr:
		return "IA"
	case TypeBoolArr:
		return "BA"
	case TypeStringArr:
		return "SA"
	default:
		panic("VarName: unknown type " + t.String())
	}
}

func (t Type) String() string {
	switch t {
	case TypeInt:
		return "int"
	case TypeBool:
		return "bool"
	case TypeString:
		return "string"
	case TypeIntArr:
		return "intArr"
	case TypeBoolArr:
		return "boolArr"
	case TypeStringArr:
		return "stringArr"
	default:
		return "<unknown type>"
	}
}
