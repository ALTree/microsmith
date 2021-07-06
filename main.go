package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"sync/atomic"
	"time"

	"github.com/ALTree/microsmith/microsmith"
)

var BuildCount int64
var CrashCount int64
var KnownCount int64

var (
	archF      = flag.String("arch", "", "GOARCHs to fuzz (comma separated list)")
	debugF     = flag.Bool("debug", false, "Run in debug mode")
	multiPkgF  = flag.Bool("multipkg", false, "Generate multipkg programs")
	nooptF     = flag.Bool("noopt", false, "Compile with optimizations disabled")
	pF         = flag.Uint64("p", 1, "Number of workers")
	raceF      = flag.Bool("race", false, "Compile with -race")
	ssacheckF  = flag.Bool("ssacheck", false, "Compile with -d=ssa/check/on")
	toolchainF = flag.String("bin", "", "Go toolchain to fuzz")
	workdirF   = flag.String("work", "work", "Workdir for the fuzzing process")
)

var archs []string

func main() {

	flag.Parse()

	if *debugF {
		debugRun()
		os.Exit(0)
	}

	if *toolchainF == "" {
		fmt.Println("-bin must be set")
		os.Exit(2)
	}

	if *raceF && runtime.GOOS == "windows" {
		fmt.Println("-race fuzzing is not supported on Windows")
		os.Exit(2)
	}

	tc := guessToolchain(*toolchainF)
	if tc == "gc" && *archF == "" {
		fmt.Println("-arch must be set when fuzzing gc")
		os.Exit(2)
	}
	if tc != "gc" && *archF != "" {
		fmt.Println("-arch must not be set when not fuzzing gc")
		os.Exit(2)
	}

	if _, err := os.Stat(*toolchainF); os.IsNotExist(err) {
		fmt.Printf("toolchain %v does not exist\n", *toolchainF)
		os.Exit(2)
	}

	fz := microsmith.FuzzOptions{*toolchainF, *nooptF, *raceF, *ssacheckF}

	archs = strings.Split(*archF, ",")

	if tc == "gc" {
		for _, a := range archs {
			installDeps(a, fz)
		}
	}
	if *ssacheckF {
		fmt.Printf("ssacheck [seed = %v]\n", microsmith.CheckSeed)
	}

	// Create workdir if not already there
	if _, err := os.Stat(*workdirF); os.IsNotExist(err) {
		err := os.MkdirAll(*workdirF, os.ModePerm)
		if err != nil {
			fmt.Println(err)
			os.Exit(2)
		}
	}

	startTime := time.Now()

	for i := uint64(1); i <= *pF; i++ {
		go Fuzz(i, fz)
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
	// regexp.MustCompile("illegal combination SRA"),
}

func Fuzz(id uint64, fz microsmith.FuzzOptions) {
	rs := rand.New(
		rand.NewSource(int64(0xfaff0011 * id * uint64(time.Now().UnixNano()))),
	)
	conf := microsmith.RandConf(rs)
	conf.MultiPkg = *multiPkgF

	counter := 0
	for {
		counter++
		if counter == 30 {
			conf = microsmith.RandConf(rs)
			conf.MultiPkg = *multiPkgF
			counter = 0
		}

		gp := microsmith.NewProgram(rs, conf)

		err := gp.Check()
		if err != nil {
			fmt.Printf("Program failed typechecking: %s\n%s", err, gp)
			os.Exit(2)
		}

		err = gp.WriteToDisk(*workdirF)
		if err != nil {
			fmt.Printf("Could not write program to file: %s", err)
			os.Exit(2)
		}

		var known bool
		for _, arch := range archs {
			timeout := time.AfterFunc(
				60*time.Second,
				func() {
					gp.MoveCrasher()
					fmt.Printf("%v took more than 60s to compile [GOARCH=%v]\n", gp.Name(), arch)
					os.Exit(2)
				},
			)
			out, err := gp.Compile(arch, fz)
			timeout.Stop()

			if err != nil {
				for _, crash := range crashWhitelist {
					if crash.MatchString(out) {
						known = true
						break
					}
				}

				if known {
					atomic.AddInt64(&KnownCount, 1)
					break
				}

				atomic.AddInt64(&CrashCount, 1)
				if arch != "" {
					fmt.Printf("-- CRASH (%v) ----------------------------------------------\n", arch)
				} else {
					fmt.Printf("-- CRASH ---------------------------------------------------\n")
				}
				fmt.Println(fiveLines(out))
				fmt.Println("------------------------------------------------------------")
				gp.MoveCrasher()
				break
			}
		}

		atomic.AddInt64(&BuildCount, 1)
		gp.DeleteSource()
	}
}

func debugRun() {
	rs := rand.New(rand.NewSource(int64(uint64(time.Now().UnixNano()))))
	conf := microsmith.ProgramConf{
		StmtConf:       microsmith.StmtConf{MaxStmtDepth: 2},
		SupportedTypes: microsmith.AllTypes,
		MultiPkg:       *multiPkgF,
		FuncNum:        2,
	}
	gp := microsmith.NewProgram(rs, conf)
	err := gp.Check()
	fmt.Println(gp)
	if err != nil {
		fmt.Printf("Program failed typechecking\n%s\n", err)
		os.Exit(2)
	}
}

func installDeps(arch string, fz microsmith.FuzzOptions) {
	var cmd *exec.Cmd
	if fz.Race {
		cmd = exec.Command(fz.Toolchain, "install", "-race", "std")
	} else {
		cmd = exec.Command(fz.Toolchain, "install", "std")
	}

	goos := "linux"
	if arch == "wasm" {
		goos = "js"
	}
	if arch == "386sf" {
		arch = "386"
		cmd.Env = append(os.Environ(), "GO386=softfloat")
	}
	cmd.Env = append(os.Environ(), "GOOS="+goos, "GOARCH="+arch)

	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("Installing dependencies failed with error:\n----\n%s\n%s\n----\n", out, err)
		os.Exit(2)
	}
}

func guessToolchain(toolchain string) string {
	switch {
	case strings.Contains(*toolchainF, "gcc"):
		return "gcc"
	case strings.Contains(*toolchainF, "tinygo"):
		return "tinygo"
	default:
		return "gc"
	}
}

func fiveLines(s string) string {
	nl := 0
	for i := range s {
		if s[i] == '\n' {
			nl++
		}
		if nl == 5 {
			return s[:i] + "\n..."
		}
	}
	return s
}
