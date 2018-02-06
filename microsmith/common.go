package microsmith

import (
	"math/rand"
)

func RandString(strings []string) string {
	return strings[rand.Intn(len(strings))]
}

func RandType(ts []Type) Type {
	return ts[rand.Intn(len(ts))]
}

// RandIndex takes a list of probs and a random float64 in [0,1) and
// returns an i in [0, len(probs)] with chance probs[i]/sum(probs).
func RandIndex(probs []float64, rand float64) int {
	if rand >= 1.0 {
		panic("RandIndex: rand > 1")
	}

	ps := make([]float64, len(probs))

	// normalize
	sum := 0.0
	for i := range probs {
		sum += probs[i]
	}
	for i := range probs {
		ps[i] = probs[i] / sum
	}

	// progressive sum
	progSum := ps[0]
	for i := 0; true; i++ {
		if rand <= progSum {
			return i
		}
		progSum += ps[i+1]
	}

	panic("unreachable")
}

// RandSplit splits integer n in p parts that sums up to n.
func RandSplit(n, p int) []int {
	if p < 1 || n < 1 {
		panic("RandSplit: parts < 1 or n < 1")
	}
	if p == 1 {
		return []int{n}
	}
	// p > 1

	if n < p { // See Issue #23
		res := make([]int, p)
		for ; n > 0; n-- {
			randIndex := rand.Intn(len(res))
			res[randIndex]++
		}
		return res
	}

	ta := make([]float64, p)

	// first, fill ta with random floats
	sum := 0.0
	for i := range ta {
		f := rand.Float64()
		sum += f
		ta[i] = f
	}

	// normalize ta so that sum(ta) = 1.0
	for i := range ta {
		ta[i] /= sum
	}

	res := make([]int, p)
	resS := 0
	for i := range res {
		res[i] = int(ta[i] * float64(n))
		resS += res[i]
	}

	// distribute what's left (1 each)
	for i := 0; resS < n; i++ {
		res[i] += 1
		resS++
	}

	return res
}
