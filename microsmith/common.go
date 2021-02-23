package microsmith

import (
	"math/rand"
	"strconv"
)

// Returns a random ASCII string of length 0-16
func RandString() string {
	rs := []string{
		`""`,
		`"a"`,
		`"ab"`,
		`"abc"`,
		`"abcd"`,
		`"abcde"`,
		`"abcdef"`,
		`"abcdefg"`,
		`"abcdefgh"`,
		`"abcdefghi"`,
		`"abcdefghij"`,
		`"abcdefghijk"`,
		`"abcdefghijkl"`,
		`"abcdefghijklm"`,
		`"abcdefghijklmn"`,
		`"abcdefghijklmno"`,
		`"abcdefghijklmnop"`,
	}
	return rs[rand.Intn(len(rs))]
}

// returns a random rune literal
func RandRune() string {
	switch rand.Intn(3) {
	case 0:
		// single character within the quotes: 'a'
		return "'" + string(byte('0'+rand.Intn('Z'-'0'))) + "'"
	case 1:
		// \x followed by exactly two hexadecimal digits: \x4f
		return "'\\x" + strconv.FormatInt(0x10+int64(rand.Intn(0xff-0x10)), 16) + "'"
	case 2:
		// \u followed by exactly four hexadecimal digits: \u3b7f
		return "'\\u" + strconv.FormatInt(0x1000+int64(rand.Intn(0xd000-0x1000)), 16) + "'"
	default:
		panic("unreachable")
	}
}

func RandType(ts []Type) Type {
	return ts[rand.Intn(len(ts))]
}

func IsEnabled(typ string, conf ProgramConf) bool {
	for _, t := range conf.SupportedTypes {
		if t.Name() == typ {
			return true
		}
	}
	return false
}
