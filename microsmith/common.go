package microsmith

import (
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
