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
	pF         = flag.Uint64("p", 1, "The number of workers")
	raceF      = flag.Bool("race", false, "Compile with -race")
	ssacheckF  = flag.Bool("ssacheck", false, "Compile with -d=ssa/check/on")
	toolchainF = flag.String("bin", "go", "The go toolchain to fuzz")
	workdirF   = flag.String("work", "work", "Workdir for the fuzzing process")
)

var lg *log.Logger
var archs []string

func init() {
	lg = log.New(os.Stderr, "[ERROR] ", log.Lshortfile)
}

func main() {

	flag.Parse()

	archs = strings.Split(*archF, ",")

	nWorkers := *pF
	if *debugF {
		nWorkers = 1
		microsmith.FuncCount = 1
	} else {
		if !(strings.Contains(*toolchainF, "gcc") || strings.Contains(*toolchainF, "tinygo")) {
			for _, a := range archs {
				installDeps(a)
			}
		}
		if *ssacheckF {
			fmt.Printf("ssacheck [seed = %v]\n", microsmith.CheckSeed)
		}
	}

	// Create workdir if not already there
	if _, err := os.Stat(*workdirF); os.IsNotExist(err) {
		err := os.MkdirAll(*workdirF, os.ModePerm)
		if err != nil {
			lg.Fatalf("%v", err)
		}
	}

	startTime := time.Now()

	for i := uint64(1); i <= nWorkers; i++ {
		go Fuzz(i)
	}

	ticker := time.Tick(30 * time.Second)
	for _ = range ticker {
		fmt.Printf("Built %4d (%5.1f/min)  |  %v crashes",
			atomic.LoadInt64(&BuildCount),
			float64(atomic.LoadInt64(&BuildCount))/time.Since(startTime).Minutes(),
			atomic.LoadInt64(&CrashCount),
		)
		if kc := atomic.LoadInt64(&KnownCount); kc == 0 {
			fmt.Print("\n")
		} else {
			fmt.Printf("  (known: %v)\n", kc)
		}
	}

	select {}
}

var crashWhitelist = []*regexp.Regexp{
	regexp.MustCompile("illegal combination SRA"),
}

func Fuzz(workerID uint64) {
	rs := rand.New(
		rand.NewSource(int64(0xfaff0011 * workerID * uint64(time.Now().UnixNano()))),
	)
	conf := microsmith.RandConf(rs)

	counter := 0
	for {
		counter++
		if counter == 16 {
			conf = microsmith.RandConf(rs)
			counter = 0
		}

		gp, err := microsmith.NewProgram(rs, conf)
		if err != nil {
			lg.Fatalf("Bad Conf: %s", err)
		}

		err = gp.Check()
		if err != nil {
			if *debugF {
				fmt.Println(gp)
				fmt.Printf("Program failed typechecking with error:\n%s\n", err)
				os.Exit(2)
			} else {
				lg.Fatalf("Program failed typechecking: %s\n%s", err, gp)
			}
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

		var known bool
		for _, arch := range archs {
			timeout := time.AfterFunc(
				60*time.Second,
				func() { lg.Fatalf("Program took more than 60s to compile\n") },
			)
			out, err := gp.Compile(*toolchainF, arch, *nooptF, *raceF, *ssacheckF)
			timeout.Stop()
			atomic.AddInt64(&BuildCount, 1)

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
					fmt.Printf("%v compilation failed\n", arch)
					fmt.Println("------------------------------------------------------------")
					fmt.Print(out)
					fmt.Println("------------------------------------------------------------")
					gp.MoveCrasher()
					fmt.Println("Crasher was saved.")
					break
				}
			}
		}

		gp.DeleteSource()
	}
}

func installDeps(goarch string) {
	cmd := exec.Command(*toolchainF, "install", "std")
	goos := "linux"
	if goarch == "wasm" {
		goos = "js"
	}
	if goarch == "386sf" {
		goarch = "386"
		cmd.Env = append(os.Environ(), "GO386=softfloat")
	}
	cmd.Env = append(os.Environ(), "GOOS="+goos, "GOARCH="+goarch)

	out, err := cmd.CombinedOutput()
	if err != nil {
		lg.Fatalf("Installing dependencies failed with error:\n  ----\n  %s\n  %s\n  ----\n", out, err)
	}
}
