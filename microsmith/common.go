package microsmith

import (
	"fmt"
	"math/rand"
)

func RandString(strings []string) string {
	return strings[rand.Intn(len(strings))]
}

// RandIndex takes a list of probs and a random float64 in [0,1) and
// returns an i in [0, len(probs)] with chance probs[i]/sum(probs).
func RandIndex(probs []float64, rand float64) int {
	if rand >= 1.0 {
		panic("RandIndex: rand > 1")
	}

	// normalize
	sum := 0.0
	for i := range probs {
		sum += probs[i]
	}
	for i := range probs {
		probs[i] /= sum
	}

	// progressive sum
	progSum := probs[0]
	for i := 0; true; i++ {
		if rand <= progSum {
			return i
		}
		progSum += probs[i+1]
	}

	panic("unreachable")
}

// RandSplit splits integer n in 'parts' parts that sums to n. It is
// guaranteeed that each split is at least 1.
//
// Example :RandSplit(8, 3) may return {1, 4, 3} or any other length-3
// array that sums to 8.
func RandSplit(n, parts int) []int {

	ta := make([]float64, parts)

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

	// now we should set each res[i] = int(n*ta[i]), but that does not
	// guarantee res[i] > 0, so we pretend we are splitting n - parts
	// (and not n), so that we can add a fixed +1 to each res[i].
	// Also to avoid rounding errors instead of setting all res, we
	// set all except the last one, and we later set the last one to
	// n - upTo.
	res := make([]int, parts)
	upTo := 0
	for i := 0; i < parts-1; i++ {
		res[i] = 1 + int(ta[i]*float64(n-parts))
		upTo += res[i]
	}

	res[parts-1] = n - upTo

	// sanity check
	// TODO: disable
	sumCheck := 0
	for i := range res {
		sumCheck += res[i]
	}
	if len(res) != parts || sumCheck != n {
		fmt.Println(">>", n, parts, res, sumCheck)
		panic("RandSplit: bad split")
	}

	return res
}
