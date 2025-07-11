package microsmith

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/printer"
	"go/token"
	"go/types"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"strings"
)

// ----------------------------------------------------------------
//   ProgramBuilder
// ----------------------------------------------------------------

type ProgramBuilder struct {
	conf ProgramConf
	pkgs []*PackageBuilder
}

func NewProgramBuilder(conf ProgramConf) *ProgramBuilder {
	return &ProgramBuilder{conf: conf}
}

func (pb *ProgramBuilder) NewPackage(pkg string) *Package {
	db := NewPackageBuilder(pb.conf, pkg, pb)
	pb.pkgs = append(pb.pkgs, db)
	var buf bytes.Buffer
	printer.Fprint(&buf, token.NewFileSet(), db.File())
	src := bytes.ReplaceAll(buf.Bytes(), []byte("func "), []byte("\nfunc "))
	return &Package{name: pkg, source: src}
}

// ----------------------------------------------------------------
//   Program
// ----------------------------------------------------------------

type Program struct {
	workdir string     // directory where the Program files are written
	pkgs    []*Package // the program's packages
	id      uint64     // random id used in the Program's filenames
}

type Package struct {
	name     string
	source   []byte
	filename string
	path     *os.File
}

type BuildOptions struct {
	Toolchain             string
	Noopt, Race, Ssacheck bool
	Experiment            string
}

var CheckSeed int

func init() {
	CheckSeed = rand.Int() % 1e5
}

func NewProgram(conf ProgramConf) *Program {
	pb := NewProgramBuilder(conf)
	pg := &Program{
		id:   rand.Uint64(),
		pkgs: make([]*Package, 0),
	}

	if conf.MultiPkg {
		pg.pkgs = append(pg.pkgs, pb.NewPackage(fmt.Sprintf("a_%d", pg.id)))
	}

	// main has to be last because it calls functions from the other
	// packages, which need to already exist in order for it to see
	// them.
	pg.pkgs = append(pg.pkgs, pb.NewPackage("main"))

	return pg
}

func (prog *Program) WriteToDisk(path string) error {
	prog.workdir = path
	for i, pkg := range prog.pkgs {
		var fileName string
		if pkg.name == "main" {
			fileName = fmt.Sprintf("main_%v.go", prog.id)
		} else {
			fileName = fmt.Sprintf("%v.go", pkg.name)
		}

		fh, err := os.Create(path + "/" + fileName)
		defer fh.Close()
		if err != nil {
			return err
		}

		fh.Write(pkg.source)
		prog.pkgs[i].filename = fileName
		prog.pkgs[i].path = fh
	}
	return nil
}

// Check uses go/parser and go/types to parse and typecheck gp
// in-memory.
func (prog *Program) Check() error {
	if len(prog.pkgs) > 1 {
		if _, err := os.Stat("work"); os.IsNotExist(err) {
			err := os.MkdirAll("work", os.ModePerm)
			if err != nil {
				return err
			}
		}
		defer func() { os.RemoveAll("work") }()

		prog.WriteToDisk("work")
		tc := "go"
		if bin := os.Getenv("GO_TC"); bin != "" {
			tc = bin
		}
		msg, err := prog.Compile("amd64", BuildOptions{Toolchain: tc})
		if err != nil {
			return errors.Join(err, errors.New(msg))
		}
		return nil
	}

	pkg, fset := prog.pkgs[0], token.NewFileSet()
	f, err := parser.ParseFile(fset, pkg.filename, pkg.source, 0)
	if err != nil {
		return err // parse error
	}

	conf := types.Config{Importer: importer.Default()}
	_, err = conf.Check(pkg.filename, fset, []*ast.File{f}, nil)
	if err != nil {
		return err // typecheck error
	}

	return nil
}

// Compile uses the given toolchain to build gp. It assumes that gp's
// source is already written to disk by Program.WriteToDisk.
//
// If the compilation subprocess exits with an error code, Compile
// returns the error message printed by the toolchain and the
// subprocess error code.
func (prog *Program) Compile(arch string, bo BuildOptions) (string, error) {
	if len(prog.pkgs) == 0 {
		return "", errors.New("Program has no packages")
	}

	baseName := fmt.Sprintf("%v", prog.id)
	arcName := "main_" + baseName + ".o"

	switch {

	case strings.Contains(bo.Toolchain, "gccgo"):
		oFlag := "-O2"
		if bo.Noopt {
			oFlag = "-Og"
		}
		cmd := exec.Command(bo.Toolchain, oFlag, "-o", arcName, "main_"+baseName+".go")
		cmd.Dir = prog.workdir
		out, err := cmd.CombinedOutput()
		if err != nil {
			return string(out), err
		}

	case strings.Contains(bo.Toolchain, "tinygo"):
		oFlag := "s"
		if bo.Noopt {
			oFlag = "0"
		}
		cmd := exec.Command(bo.Toolchain, "build", "-opt", oFlag, "-o", arcName, "main_"+baseName+".go")
		cmd.Dir = prog.workdir
		out, err := cmd.CombinedOutput()
		if err != nil {
			return string(out), err
		}

	default:

		// Setup env variables
		env := os.Environ()
		if arch == "wasm" {
			env = append(env, "GOOS=js")
		} else {
			env = append(env, "GOOS=linux")
		}

		if arch == "386sf" {
			env = append(env, "GOARCH=386", "GO386=softfloat")
		} else if arch == "amd64_v3" {
			env = append(env, "GOARCH=amd64", "GOAMD64=v3")
		} else {
			env = append(env, "GOARCH="+arch)
		}

		if exp := bo.Experiment; exp != "" {
			env = append(env, "GOEXPERIMENT="+exp)
		}

		// Setup compile args
		buildArgs := []string{"tool", "compile"}
		if bo.Race {
			buildArgs = append(buildArgs, "-race")
		}
		if bo.Noopt {
			buildArgs = append(buildArgs, "-N", "-l")
		}
		if bo.Ssacheck {
			cs := fmt.Sprintf("-d=ssa/check/seed=%v", CheckSeed)
			buildArgs = append(buildArgs, cs)
		}

		// Compile
		for _, pkg := range prog.pkgs {
			var cmdArgs []string
			buildArgs = append(buildArgs, []string{"-p", pkg.name}...)
			if pkg.name == "main" {
				cmdArgs = append(buildArgs, "-I=.")
				cmdArgs = append(cmdArgs, pkg.filename)
			} else {
				cmdArgs = append(buildArgs, pkg.filename)
			}

			cmd := exec.Command(bo.Toolchain, cmdArgs...)
			cmd.Dir, cmd.Env = prog.workdir, env
			out, err := cmd.CombinedOutput()
			if err != nil {
				return string(out), err
			}
		}

		// Setup link args
		linkArgs := []string{"tool", "link", "-L=."}
		if bo.Race {
			linkArgs = append(linkArgs, "-race")
		}
		linkArgs = append(linkArgs, "-o", baseName, arcName)

		// Link
		cmd := exec.Command(bo.Toolchain, linkArgs...)
		cmd.Dir, cmd.Env = prog.workdir, env
		out, err := cmd.CombinedOutput()
		if err != nil {
			return string(out), err
		}
	}

	prog.DeleteBinaries()
	return "", nil
}

// DeleteBinaries deletes any binary file written on disk.
func (prog *Program) DeleteBinaries() {
	basePath := prog.workdir + "/"
	for _, pkg := range prog.pkgs {
		var err error
		if pkg.name == "main" {
			err = os.Remove(basePath + fmt.Sprintf("main_%d.o", prog.id))
		} else {
			err = os.Remove(basePath + pkg.name + ".o")
		}
		if err != nil {
			log.Printf("could not remove %s: %s", basePath+"_"+pkg.name+".o", err)
		}

	}

	// ignore error since some toolchains don't write a binary
	_ = os.Remove(basePath + fmt.Sprintf("%v", prog.id))
}

// DeleteSource deletes all gp files.
func (gp Program) DeleteSource() {
	for _, file := range gp.pkgs {
		_ = os.Remove(file.path.Name())
	}
}

// Move gp's files in a workdir subfolder named "crash".
func (gp Program) MoveCrasher() {
	fld := gp.workdir + "/crash"
	if _, err := os.Stat(fld); os.IsNotExist(err) {
		err := os.Mkdir(fld, os.ModePerm)
		if err != nil {
			fmt.Printf("Could not create crash folder: %v", err)
			os.Exit(2)
		}
	}

	for _, pkg := range gp.pkgs {
		err := os.Rename(pkg.path.Name(), fld+"/"+pkg.filename)
		if err != nil {
			fmt.Printf("Could not move crasher: %v", err)
			os.Exit(2)
		}
	}
}

func (prog *Program) String() string {
	var res string
	for _, pkg := range prog.pkgs {
		res += string(pkg.source)
		if len(prog.pkgs) > 1 {
			res += "\n--------------------------------------------------\n"
		}
	}
	return res
}

func (prog *Program) Name() string {
	return fmt.Sprintf("%v", prog.id)
}
