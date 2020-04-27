package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	"github.com/ALTree/microsmith/microsmith"
)

var BuildCount int64
var CrashCount int64
var KnownCount int64

var (
	archF      = flag.String("arch", "amd64", "The GOARCH to fuzz")
	debugF     = flag.Bool("debug", false, "Run fuzzer in debug mode")
	nooptF     = flag.Bool("noopt", false, "Compile with optimizations disabled")
	pF         = flag.Int("p", 1, "The number of workers")
	raceF      = flag.Bool("race", false, "Compile with -race")
	ssacheckF  = flag.Bool("ssacheck", false, "Compile with -d=ssa/check/on")
	toolchainF = flag.String("bin", "go", "The go toolchain to fuzz")
	workdirF   = flag.String("work", "work", "Workdir for the fuzzing process")
)

var lg *log.Logger

func init() {
	lg = log.New(os.Stderr, "[ERROR] ", log.Lshortfile)
}

func main() {

	flag.Parse()

	nWorkers := *pF
	if *debugF {
		nWorkers = 1
		microsmith.Nfuncs = 1
	} else {
		if !(strings.Contains(*toolchainF, "gcc") || strings.Contains(*toolchainF, "tinygo")) {
			installDeps()
		}
		s := "Start fuzzing"
		if *ssacheckF {
			s += fmt.Sprintf(" [seed %v]", microsmith.CheckSeed)
		}
		fmt.Println(s)
	}

	// Create workdir if not already there
	if _, err := os.Stat(*workdirF); os.IsNotExist(err) {
		err := os.MkdirAll(*workdirF, os.ModePerm)
		if err != nil {
			lg.Fatalf("%v", err)
		}
	}

	for i := 0; i < nWorkers; i++ {
		go Fuzz(7 + i*117)
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
	// regexp.MustCompile("bvbulkalloc too big"),
}

func Fuzz(seed int) {
	rand := rand.New(rand.NewSource(int64(seed)))
	conf := microsmith.RandConf()

	counter := 0
	for {
		counter++
		if counter == 16 {
			conf = microsmith.RandConf()
			counter = 0
		}

		gp, err := microsmith.NewProgram(rand.Int63(), conf)
		if err != nil {
			lg.Fatalf("Bad Conf: %s", err)
		}

		err = gp.Check()
		if err != nil {
			lg.Fatalf("Program failed typechecking: %s\n%s", err, gp)
		}

		if *debugF {
			// TODO: print gp stats too(?)
			fmt.Println(gp)
			os.Exit(0)
		}

		err = gp.WriteToFile(*workdirF)
		if err != nil {
			lg.Fatalf("Could not write program to file: %s", err)
		}

		// Crash Fuzzer if compilation takes more than 60s
		timeout := time.AfterFunc(
			60*time.Second,
			func() { lg.Fatalf("> 60s compilation time for\n%s\n", gp) },
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
				fmt.Println("Program compilation failed:")
				fmt.Println("------------------------------------------------------------")
				fmt.Print(out)
				fmt.Println("------------------------------------------------------------")
				gp.MoveCrasher()
				fmt.Printn("Crasher was saved.")
			}
		}

		atomic.AddInt64(&BuildCount, 1)
		if err == nil || known {
			gp.DeleteFile()
		}

	}
}

func installDeps() {
	cmd := exec.Command(*toolchainF, "install", "math")
	goos := "linux"
	if *archF == "wasm" {
		goos = "js"
	}
	cmd.Env = append(os.Environ(), "GOOS="+goos, "GOARCH="+*archF)

	fmt.Printf("Installing dependencies for %s/%s\n", goos, *archF)

	out, err := cmd.CombinedOutput()
	if err != nil {
		lg.Fatalf("Installing dependencies failed with error:\n  ----\n  %s\n  %s\n  ----\n", out, err)
	}
}
