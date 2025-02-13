package main

import (
	"bufio"
	"bytes"
	"errors"
	"flag"
	"go/types"
	"io"
	"log"
	"maps"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"strings"

	"golang.org/x/tools/go/packages"
)

// TODO: Sort output order
// TODO: Add comments from source type
// TODO: Remove all panics

func main() {
	wd, err := os.Getwd()
	var dir string
	var filePrefix string
	var outputPkgName string
	flag.StringVar(&dir, "C", wd, "directory of package to generate in")
	flag.StringVar(&filePrefix, "f", "interfaces", "file name prefix for the generated files")
	flag.StringVar(&outputPkgName, "pkg", "", "name of the output package")
	flag.Parse()

	if outputPkgName == "" {
		log.Fatal("missing -pkg param")
	}

	pkgs := map[string]map[string]string{}
	for _, v := range flag.Args() {
		pkgPathSplit := strings.Split(v, ":")
		pkgPath := pkgPathSplit[0]
		pkg := pkgs[pkgPath]
		if pkg == nil {
			pkg = map[string]string{}
			pkgs[pkgPath] = pkg
		}
		mappings := strings.Split(pkgPathSplit[1], ",")
		for _, mapping := range mappings {
			rename := strings.Split(mapping, "=>")
			pkg[rename[0]] = rename[1]
		}
	}

	normalName := filePrefix + ".go"
	impureName := filePrefix + "_impure.go"
	cfg := packages.Config{
		Dir:  dir,
		Mode: packages.LoadAllSyntax,
	}
	loadedPkgs, err := packages.Load(&cfg, slices.Sorted(maps.Keys(pkgs))...)
	if err != nil {
		panic(err)
	}

	// Load all the types in the source packages and map them
	// to the new target names.
	loadedTypes := map[string]map[string]*types.Named{}
	for _, loadedPkg := range loadedPkgs {
		scope := loadedPkg.Types.Scope()
		mappings := pkgs[loadedPkg.PkgPath]
		mapped := map[string]*types.Named{}
		for sourceName, targetName := range mappings {
			t := scope.Lookup(sourceName)
			a, ok := t.Type().(*types.Named)
			if !ok {
				panic("not a named type")
			}
			mapped[targetName] = a
		}
		loadedTypes[loadedPkg.PkgPath] = mapped
	}

	// Generate the impure full interfaces.
	g := NewGen(outputPkgName, impureName, "impure")
	for pkgPath, mapped := range loadedTypes {
		for targetName, t := range mapped {
			err = g.AddInterface(t, targetName, pkgPath, nil)
			if err != nil {
				panic(err)
			}
		}
	}
	err = g.Write(dir)
	if err != nil {
		panic(err)
	}

	// Generate temporary empty interfaces.
	g2 := NewGen(outputPkgName, normalName, "!impure")
	for pkgPath, mapped := range loadedTypes {
		for targetName, t := range mapped {
			err = g2.AddInterface(t, targetName, pkgPath, map[string]bool{})
			if err != nil {
				panic(err)
			}
		}
	}
	err = g2.Write(dir)
	if err != nil {
		panic(err)
	}

	// Test building with the full interfaces. This must succeed.
	buf := &bytes.Buffer{}
	cmd := exec.Command("go", "build", "-tags", "impure")
	cmd.Dir = dir
	cmd.Stderr = os.Stderr
	cmd.Stdout = buf
	err = cmd.Run()
	if err != nil {
		panic(err)
	}

	// Test building with the empty interfaces. This should
	// fail if there are calls or uses of methods in the empty
	// interfaces.
	buf = &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	cmd = exec.Command("go", "build")
	cmd.Dir = dir
	cmd.Stderr = errBuf
	cmd.Stdout = buf
	err = cmd.Run()
	if err == nil {
		os.Exit(0)
		return
	}

	// Read the error log from the build and add in all the
	// missing methods.
	wants := map[string][]string{}
	errReader := bufio.NewReader(errBuf)
	for {
		line, err := errReader.ReadString('\n')
		if errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			panic(err)
		}
		if !strings.Contains(line, "undefined") {
			continue
		}
		res := missingMethod.FindStringSubmatch(line)
		if res == nil {
			continue
		}
		wants[res[1]] = append(wants[res[1]], res[2])
	}

	// Write out the final pure interfaces with just the
	// methods required.
	g3 := NewGen(outputPkgName, normalName, "!impure")
	for pkgPath, mapped := range loadedTypes {
		for targetName, t := range mapped {
			filter := map[string]bool{}
			for _, v := range wants[targetName] {
				filter[v] = true
			}
			err = g3.AddInterface(t, targetName, pkgPath, filter)
			if err != nil {
				panic(err)
			}
		}
	}
	err = g3.Write(dir)
	if err != nil {
		panic(err)
	}
}

var missingMethod = regexp.MustCompile(`.*\(type (\w*) has no field or method (\w*)\).*`)
