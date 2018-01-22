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
const debug = false

var BuildCount int64

func main() {

	var pFlag = flag.Int("p", 1, "number of workers")
	var archFlag = flag.String("arch", "amd64", "GOARCH to fuzz")
	flag.Parse()

	nWorkers := *pFlag
	if nWorkers < 1 || debug {
		nWorkers = 1
	}
	fmt.Printf("Fuzzing %v with %v worker(s)\n", *archFlag, nWorkers)
	for i := 0; i < nWorkers; i++ {
		go Fuzz(int64(i), *archFlag)
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
		if debug {
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

		err = gp.Compile(arch)
		if err != nil {
			log.Fatalf("Program did not compile: %s\n%s", err, gp)
		}

		gp.DeleteFile()
		atomic.AddInt64(&BuildCount, 1)
		if debug {
			os.Exit(0)
		}
	}
}
