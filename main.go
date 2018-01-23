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
	pF     = flag.Int("p", 1, "number of workers")
	archF  = flag.String("arch", "amd64", "GOARCH to fuzz")
	debugF = flag.Bool("debug", false, "run fuzzer in debug mode")
)

func main() {

	flag.Parse()

	rs := rand.New(rand.NewSource(time.Now().UnixNano()))

	nWorkers := *pF
	if nWorkers < 1 || *debugF {
		nWorkers = 1
	}
	fmt.Printf("Fuzzing GOARCH=%v with %v worker(s)\n", *archF, nWorkers)
	for i := 0; i < nWorkers; i++ {
		go Fuzz(rs.Int63(), *archF)
	}

	ticker := time.Tick(5 * time.Second)
	for _ = range ticker {
		log.Printf("Build: %4d  [crash: %v, known: %v]\n",
			atomic.LoadInt64(&BuildCount),
			atomic.LoadInt64(&CrashCount),
			atomic.LoadInt64(&KnownCount))
	}

	select {}
}

var crashWhitelist = []*regexp.Regexp{
	regexp.MustCompile("internal compiler error: panic during layout"),
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

		out, err := gp.Compile(*archF)
		if err != nil {
			var known bool
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
				log.Fatalf("Program did not compile:\n%s\n%s\n%s", out, err, gp)
			}
		}

		gp.DeleteFile()
		atomic.AddInt64(&BuildCount, 1)
		if *debugF {
			os.Exit(0)
		}
	}
}
