package microsmith_test

import (
	"math/rand"
	"testing"

	"github.com/ALTree/microsmith/microsmith"
)

const WorkDir = "../work/"

func TestProgramGeneration(t *testing.T) {
	rand := rand.New(rand.NewSource(42))

	// check generated programs with go/types (in-memory)
	t.Run("GoTypesCheck", func(t *testing.T) {
		for i := 0; i < 2000; i++ {
			gp := microsmith.NewGoProgram(rand.Int63())
			err := gp.Check()
			if err != nil {
				t.Fatalf("Program failed typechecking: %s\n%s", err, gp)
			}
		}
	})

	// check generated programs with gc (from file)
	t.Run("GcCheck", func(t *testing.T) {
		for i := 0; i < 50; i++ {
			gp := microsmith.NewGoProgram(rand.Int63())
			err := gp.WriteToFile(WorkDir)
			if err != nil {
				t.Fatalf("Could not write to file: %s", err)
			}
			err = gp.Compile()
			if err != nil {
				t.Fatalf("Program did not compile: %s\n%s", err, gp)
			}
			gp.DeleteFile()
		}
	})
}
