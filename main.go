package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"regexp"
	"sync/atomic"
	"time"

	"github.com/ALTree/microsmith/microsmith"
)

const WorkDir = "work/"

var BuildCount int64
var CrashCount int64
var KnownCount int64

var (
	pF         = flag.Int("p", 1, "number of workers")
	debugF     = flag.Bool("debug", false, "run fuzzer in debug mode")
	archF      = flag.String("arch", "amd64", "GOARCH to fuzz")
	toolchainF = flag.String("gobin", "go", "go toolchain to fuzz")
	nooptF     = flag.Bool("noopt", false, "compile with optimizations disabled")
	raceF      = flag.Bool("race", false, "compile with -race")
	ssacheckF  = flag.Bool("ssacheck", false, "compile with -d=ssa/check/on")
)

func main() {

	flag.Parse()

	rs := rand.New(rand.NewSource(time.Now().UnixNano()))

	nWorkers := *pF
	if nWorkers < 1 || *debugF {
		nWorkers = 1
		microsmith.Nfuncs = 1
	}
	fmt.Printf("Fuzzing GOARCH=%v with %v worker(s)\n", *archF, nWorkers)
	for i := 0; i < nWorkers; i++ {
		go Fuzz(rs.Int63())
	}

	ticker := time.Tick(30 * time.Second)
	for _ = range ticker {
		fmt.Printf("Build: %4d  [crash: %v, known: %v]\n",
			atomic.LoadInt64(&BuildCount),
			atomic.LoadInt64(&CrashCount),
			atomic.LoadInt64(&KnownCount))
	}

	select {}
}

var crashWhitelist = []*regexp.Regexp{
	regexp.MustCompile("bvbulkalloc too big"),
}

// Fuzz with one worker
func Fuzz(seed int64) {
	rand := rand.New(rand.NewSource(seed))

	// Start with the defaul configuration; when not in debug mode
	// we'll change program configuration once in a while.
	conf := microsmith.DefaultConf

	counter := 0
	for true {
		counter++
		if counter == 32 { // will never be > 1 in debug mode
			conf = microsmith.RandConf()
			counter = 0
		}

		gp, err := microsmith.NewGoProgram(rand.Int63(), conf)
		if err != nil {
			log.Fatalf("Bad Conf: %s", err)
		}

		err = gp.Check()
		if err != nil {
			log.Fatalf("Program failed typechecking: %s\n%s", err, gp)
		}

		if *debugF {
			fmt.Println(gp)
			os.Exit(0)
		}

		err = gp.WriteToFile(WorkDir)
		if err != nil {
			log.Fatalf("Could not write to file: %s", err)
		}

		// Interrupt and crash Fuzzer if compilation takes more than
		// 60 seconds
		timeout := time.AfterFunc(
			60*time.Second,
			func() { log.Fatalf("> 60s compilation time for\n%s\n", gp) },
		)

		out, err := gp.Compile(*toolchainF, *archF, *nooptF, *raceF, *ssacheckF)
		timeout.Stop()
		var known bool
		if err != nil {
			for _, crash := range crashWhitelist {
				if crash.MatchString(out) {
					known = true
					break
				}
			}

			if known {
				atomic.AddInt64(&KnownCount, 1)
			} else {
				atomic.AddInt64(&CrashCount, 1)
				log.Printf("[%v] program did not compile\n%s\n%s", gp.Seed, out, err)
			}
		}

		atomic.AddInt64(&BuildCount, 1)
		if err == nil || known {
			gp.DeleteFile()
		}
		if *debugF {
			os.Exit(0)
		}
	}
}
