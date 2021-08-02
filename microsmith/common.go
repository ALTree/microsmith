package microsmith

import (
	"math/rand"
	"strconv"
	"strings"
)

// Returns a random ASCII string
func RandString() string {
	const chars = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

	n := int(rand.NormFloat64()*8.0 + 12.0)
	if n < 0 {
		n = 0
	}

	sb := strings.Builder{}
	sb.Grow(n + 2)
	sb.WriteByte('"')
	for i := 0; i < n; i++ {
		sb.WriteByte(chars[rand.Int63()%int64(len(chars))])
	}
	sb.WriteByte('"')
	return sb.String()
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
