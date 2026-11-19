// mutar is a mutation report for a few named functions: it flips one operator at a time and
// runs the package's tests against the mutant through go test -overlay, so no file is changed.
//
//	go run ./tools/mutar [-t 10m] [-j 4] path/file.go:Func ...
//
// A mutant the tests do not catch is LIVED. A build failure is INVALID. A test timeout counts as
// killed. When the time budget runs out, the mutants left are not run and the exit code is 1.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type mutant struct {
	File     string
	Line     int
	Off      int
	Old, New string
}

var cambio = map[token.Token][]string{
	token.EQL: {"!="}, token.NEQ: {"=="},
	token.LSS: {">=", "<="}, token.GEQ: {"<", ">"},
	token.GTR: {"<=", ">="}, token.LEQ: {">", "<"},
	token.LAND: {"||"}, token.LOR: {"&&"},
}

// sitios lists the mutants inside the function or method fn of file.
func sitios(file string, src []byte, fn string) ([]mutant, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, file, src, 0)
	if err != nil {
		return nil, err
	}
	var body *ast.BlockStmt
	for _, d := range f.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok && fd.Name.Name == fn {
			body = fd.Body
		}
	}
	if body == nil {
		return nil, fmt.Errorf("%s: no function %s", file, fn)
	}
	var ms []mutant
	add := func(pos token.Pos, old string, news ...string) {
		p := fset.Position(pos)
		for _, n := range news {
			ms = append(ms, mutant{File: file, Line: p.Line, Off: p.Offset, Old: old, New: n})
		}
	}
	ast.Inspect(body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.BinaryExpr:
			add(x.OpPos, x.Op.String(), cambio[x.Op]...)
		case *ast.UnaryExpr:
			if x.Op == token.NOT {
				add(x.OpPos, "!", "")
			}
		case *ast.Ident:
			if x.Name == "true" || x.Name == "false" {
				add(x.Pos(), x.Name, map[string]string{"true": "false", "false": "true"}[x.Name])
			}
		}
		return true
	})
	sort.SliceStable(ms, func(i, j int) bool { return ms[i].Off < ms[j].Off })
	return ms, nil
}

func aplicar(src []byte, m mutant) []byte {
	out := append([]byte{}, src[:m.Off]...)
	out = append(out, m.New...)
	return append(out, src[m.Off+len(m.Old):]...)
}

// probar runs the tests of m's package with m in place: KILLED, LIVED, TIMEOUT, INVALID or NOT RUN.
func probar(ctx context.Context, m mutant, src []byte) string {
	if ctx.Err() != nil {
		return "NOT RUN"
	}
	tmp, err := os.MkdirTemp("", "mutar")
	if err != nil {
		return "INVALID"
	}
	defer os.RemoveAll(tmp)
	abs, _ := filepath.Abs(m.File)
	mut := filepath.Join(tmp, filepath.Base(m.File))
	ov, _ := json.Marshal(map[string]map[string]string{"Replace": {abs: mut}})
	if os.WriteFile(mut, aplicar(src, m), 0o644) != nil || os.WriteFile(filepath.Join(tmp, "ov.json"), ov, 0o644) != nil {
		return "INVALID"
	}
	out, err := exec.CommandContext(ctx, "go", "test", "-count=1", "-failfast", "-timeout=90s",
		"-overlay="+filepath.Join(tmp, "ov.json"), "./"+filepath.Dir(m.File)).CombinedOutput()
	var ee *exec.ExitError
	switch {
	case ctx.Err() != nil:
		return "NOT RUN"
	case err == nil:
		return "LIVED"
	case !errors.As(err, &ee):
		return "INVALID"
	case strings.Contains(string(out), "[build failed]") || strings.Contains(string(out), "[setup failed]"):
		return "INVALID"
	case strings.Contains(string(out), "panic: test timed out"):
		return "TIMEOUT"
	}
	return "KILLED"
}

func main() {
	budget := flag.Duration("t", 10*time.Minute, "time budget for the whole run")
	jobs := flag.Int("j", 4, "mutants tested at once")
	flag.Parse()
	ctx, cancel := context.WithTimeout(context.Background(), *budget)
	defer cancel()
	start := time.Now()
	var ms []mutant
	srcs := map[string][]byte{}
	for _, t := range flag.Args() {
		file, fn, _ := strings.Cut(t, ":")
		src, err := os.ReadFile(file)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		s, err := sitios(file, src, fn)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		srcs[file] = src
		ms = append(ms, s...)
		fmt.Printf("%s: %d mutants\n", t, len(s))
	}
	for dir := range paquetes(ms) {
		if out, err := exec.CommandContext(ctx, "go", "test", "-count=1", "./"+dir).CombinedOutput(); err != nil {
			fmt.Printf("the tests of %s fail without mutants:\n%s", dir, out)
			os.Exit(2)
		}
	}
	res := make([]string, len(ms))
	var wg sync.WaitGroup
	sem := make(chan struct{}, *jobs)
	for i := range ms {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			res[i] = probar(ctx, ms[i], srcs[ms[i].File])
		}()
	}
	wg.Wait()
	n := map[string]int{}
	for i, m := range ms {
		n[res[i]]++
		if res[i] == "LIVED" || res[i] == "INVALID" {
			fmt.Printf("%s %s:%d %s→%s\n", res[i], m.File, m.Line, m.Old, m.New)
		}
	}
	fmt.Printf("mutants %d · killed %d · timeout %d · lived %d · invalid %d · not run %d · %s\n", len(ms),
		n["KILLED"], n["TIMEOUT"], n["LIVED"], n["INVALID"], n["NOT RUN"], time.Since(start).Round(time.Second))
	if n["NOT RUN"] > 0 {
		os.Exit(1)
	}
}

func paquetes(ms []mutant) map[string]bool {
	d := map[string]bool{}
	for _, m := range ms {
		d[filepath.Dir(m.File)] = true
	}
	return d
}
