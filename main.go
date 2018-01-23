package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"sync/atomic"
	"time"

	"github.com/ALTree/microsmith/microsmith"
)

const WorkDir = "work/"

var BuildCount int64

var (
	pF     = flag.Int("p", 1, "number of workers")
	archF  = flag.String("arch", "amd64", "GOARCH to fuzz")
	debugF = flag.Bool("debug", false, "run fuzzer in debug mode")
)

func main() {

	flag.Parse()

	if *debugF {
		rand := rand.New(rand.NewSource(time.Now().UnixNano()))
		Fuzz(rand.Int63(), *archF)
	}

	nWorkers := *pF
	if nWorkers < 1 || *debugF {
		nWorkers = 1
	}
	fmt.Printf("Fuzzing %v with %v worker(s)\n", *archF, nWorkers)
	for i := 0; i < nWorkers; i++ {
		go Fuzz(int64(i), *archF)
	}

	ticker := time.Tick(3 * time.Second)
	for _ = range ticker {
		log.Printf("Build: %v\n", atomic.LoadInt64(&BuildCount))
	}

	select {}
}

// Fuzz with one worker
func Fuzz(seed int64, arch string) {
	rand := rand.New(rand.NewSource(seed))
	for true {
		gp := microsmith.NewGoProgram(rand.Int63())
		if *debugF {
			fmt.Println(gp)
		}

		err := gp.Check()
		if err != nil {
			log.Fatalf("Program failed typechecking: %s\n%s", err, gp)
		}

		err = gp.WriteToFile(WorkDir)
		if err != nil {
			log.Fatalf("Could not write to file: %s", err)
		}

		err = gp.Compile(*archF)
		if err != nil {
			log.Fatalf("Program did not compile: %s\n%s", err, gp)
		}

		gp.DeleteFile()
		atomic.AddInt64(&BuildCount, 1)
		if *debugF {
			os.Exit(0)
		}
	}
}
