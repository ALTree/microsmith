package microsmith

import "testing"

func TestRandSplit(t *testing.T) {

	for n := 1; n < 1000; n++ {
		for parts := 1; parts <= 10; parts++ {
			split := RandSplit(n, parts)
			if len(split) != parts {
				t.Fatalf("len(RandSplit(%v,%v)) = %v, wanted %v",
					n, parts, len(split), parts)
			}
			sum := 0
			for i := range split {
				if split[i] < 0 {
					t.Fatalf("RandSplit(%v,%v) = %v has non-positive element",
						n, parts, split)
				}
				sum += split[i]
			}
			if sum != n {
				t.Fatalf("sum(RandSplit(%v,%v)) = %v, wanted %v",
					n, parts, sum, n)
			}

			// t.Logf("RandSplit(%v,%v) = %v", n, parts, split)
		}
	}
}
