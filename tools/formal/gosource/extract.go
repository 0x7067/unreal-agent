// Command extract recognizes a deliberately small, total, pure Go subset.
// It is trusted parsing/translation tooling, not a verified Go compiler.
package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/constant"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type target struct{ ID, File, Name, Receiver string }

var targets = []target{
	{"local_finished", "harness/operation/local_manager.go", "localOperationFinished", ""},
	{"primitive_completed", "harness/operation/local_manager.go", "localPrimitiveCompleted", ""},
	{"remote_transition", "harness/operation/remote_job_handler.go", "validRemoteJobStatusTransition", ""},
	{"coordinator_terminal", "harness/coordinator/loop.go", "operationIsTerminal", ""},
	{"taskgraph_terminal", "harness/taskgraph/runtime.go", "terminal", ""},
	{"shell_output", "harness/taskgraph/runtime.go", "shellOutputPhase", ""},
	{"activity_terminal", "cmd/internal/subagent/activity.go", "activityTerminal", ""},
	{"stored_terminal", "harness/sessionstore/localfile/state.go", "terminalOperationStatus", ""},
	{"worker_terminal", "cmd/internal/workerruntime/run.go", "terminal", ""},
	{"workload_terminal", "cmd/internal/workerruntime/workload_registry.go", "terminalOperation", ""},
	{"reasoning_valid", "harness/llm/model.go", "Valid", "ReasoningEffort"},
}

type node struct {
	Kind  string  `json:"kind"`
	Value bool    `json:"value,omitempty"`
	Index int     `json:"index,omitempty"`
	Bytes []int   `json:"bytes,omitempty"`
	Args  []*node `json:"args,omitempty"`
}
type result struct {
	ID           string   `json:"id"`
	File         string   `json:"file"`
	Name         string   `json:"name"`
	Receiver     string   `json:"receiver"`
	Dependencies []string `json:"dependencies"`
	Inputs       []string `json:"inputs"`
	AST          *node    `json:"ast"`
}
type source struct {
	path string
	file *ast.File
}
type extractor struct {
	root     string
	fs       *token.FileSet
	packages map[string][]source
	inputs   map[string]int
	enums    map[string]string
	used     map[string]bool
}

func fail(format string, args ...any) { panic(fmt.Sprintf(format, args...)) }

// No symlink or path-traversal source resolution. This makes the reviewed root
// and dependency provenance unambiguous; filesystem/toolchain remain trusted.
func (e *extractor) safePath(rel string) string {
	if filepath.IsAbs(rel) || filepath.Clean(rel) != rel || rel == ".." || strings.HasPrefix(rel, "../") {
		fail("nonlocal source path %s", rel)
	}
	p := e.root
	info, err := os.Lstat(p)
	if err != nil {
		panic(err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		fail("symlink source root")
	}
	for _, part := range strings.Split(filepath.ToSlash(rel), "/") {
		if part == "." {
			continue
		}
		p = filepath.Join(p, part)
		info, err = os.Lstat(p)
		if err != nil {
			panic(err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			fail("symlink source path %s", p)
		}
	}
	return p
}
func (e *extractor) load(dir string) []source {
	if p, ok := e.packages[dir]; ok {
		return p
	}
	entries, err := os.ReadDir(e.safePath(dir))
	if err != nil {
		panic(err)
	}
	var p []source
	for _, ent := range entries {
		if ent.IsDir() || !strings.HasSuffix(ent.Name(), ".go") || strings.HasSuffix(ent.Name(), "_test.go") {
			continue
		}
		path := filepath.ToSlash(filepath.Join(dir, ent.Name()))
		f, err := parser.ParseFile(e.fs, e.safePath(path), nil, parser.ParseComments|parser.AllErrors)
		if err != nil {
			panic(err)
		}
		// Reject shadowing of the predeclared names used by the subset, and
		// dot imports which could introduce implicit bindings. Other package
		// declarations are NOT treated as verified or imported into a model.
		for _, im := range f.Imports {
			if im.Name != nil && (im.Name.Name == "." || im.Name.Name == "true" || im.Name.Name == "false" || im.Name.Name == "string" || im.Name.Name == "bool") {
				fail("dot import unsupported: %s", path)
			}
		}
		reserved := func(name string) {
			if name == "bool" || name == "string" || name == "true" || name == "false" {
				fail("shadowed predeclared name %s", name)
			}
		}
		for _, d := range f.Decls {
			switch n := d.(type) {
			case *ast.FuncDecl:
				if n.Recv == nil {
					reserved(n.Name.Name)
				}
			case *ast.GenDecl:
				for _, sp := range n.Specs {
					switch v := sp.(type) {
					case *ast.TypeSpec:
						reserved(v.Name.Name)
					case *ast.ValueSpec:
						for _, id := range v.Names {
							reserved(id.Name)
						}
					}
				}
			}
		}
		p = append(p, source{path, f})
	}
	e.packages[dir] = p
	return p
}
func (e *extractor) usable(s source) {
	e.used[s.path] = true
	// Filename build selection is just as significant as an explicit build tag.
	stem := strings.TrimSuffix(filepath.Base(s.path), ".go")
	parts := strings.Split(stem, "_")
	platforms := " aix android darwin dragonfly freebsd hurd illumos ios js linux nacl netbsd openbsd plan9 solaris wasip1 windows zos 386 amd64 amd64p32 arm armbe arm64 arm64be loong64 mips mipsle mips64 mips64le mips64p32 mips64p32le ppc ppc64 ppc64le riscv riscv64 s390 s390x sparc sparc64 wasm "
	if len(parts) > 1 && strings.Contains(platforms, " "+parts[len(parts)-1]+" ") {
		fail("platform-selected file unsupported: %s", s.path)
	}
	for _, cg := range s.file.Comments {
		for _, c := range cg.List {
			if strings.HasPrefix(c.Text, "//go:") || strings.HasPrefix(c.Text, "// +build") {
				fail("build constraints unsupported: %s", s.path)
			}
		}
	}
}
func (e *extractor) named(s source, x ast.Expr) (string, string) {
	switch n := x.(type) {
	case *ast.Ident:
		return filepath.ToSlash(filepath.Dir(s.path)), n.Name
	case *ast.SelectorExpr:
		id, ok := n.X.(*ast.Ident)
		if !ok {
			fail("unsupported qualified name")
		}
		for _, im := range s.file.Imports {
			p, err := strconv.Unquote(im.Path.Value)
			if err != nil {
				panic(err)
			}
			name := filepath.Base(p)
			if im.Name != nil {
				name = im.Name.Name
			}
			if name == id.Name {
				const prefix = "github.com/unreallabsai/unreal-agent/"
				if !strings.HasPrefix(p, prefix) {
					fail("external enum import %s", p)
				}
				return strings.TrimPrefix(p, prefix), n.Sel.Name
			}
		}
	}
	fail("unsupported name %T", x)
	return "", ""
}
func (e *extractor) typeDecl(dir, name string) (source, *ast.TypeSpec) {
	var found *ast.TypeSpec
	var src source
	for _, s := range e.load(dir) {
		for _, d := range s.file.Decls {
			g, ok := d.(*ast.GenDecl)
			if !ok || g.Tok != token.TYPE {
				continue
			}
			for _, sp := range g.Specs {
				t := sp.(*ast.TypeSpec)
				if t.Name.Name == name {
					if found != nil {
						fail("duplicate type %s", name)
					}
					found = t
					src = s
				}
			}
		}
	}
	if found == nil {
		fail("missing type %s.%s", dir, name)
	}
	e.usable(src)
	if found.Assign.IsValid() || found.TypeParams != nil {
		fail("alias/generic type unsupported")
	}
	return src, found
}
func (e *extractor) enum(s source, x ast.Expr) string {
	dir, name := e.named(s, x)
	_, t := e.typeDecl(dir, name)
	id, ok := t.Type.(*ast.Ident)
	if !ok || id.Name != "string" {
		fail("enum %s must directly have string underlying type", name)
	}
	return dir + "." + name
}
func (e *extractor) literal(s source, x ast.Expr, want string) []int {
	if _, _, ok := e.input(x); ok {
		fail("constant reference is shadowed by input")
	}
	dir, name := e.named(s, x)
	var found *ast.ValueSpec
	var src source
	var expr ast.Expr
	for _, f := range e.load(dir) {
		for _, d := range f.file.Decls {
			g, ok := d.(*ast.GenDecl)
			if !ok || g.Tok != token.CONST {
				continue
			}
			for _, sp := range g.Specs {
				v := sp.(*ast.ValueSpec)
				for i, n := range v.Names {
					if n.Name == name {
						if found != nil {
							fail("duplicate const %s", name)
						}
						if len(v.Names) != len(v.Values) || v.Type == nil {
							fail("only explicit typed literal constants supported")
						}
						found = v
						src = f
						expr = v.Values[i]
					}
				}
			}
		}
	}
	if found == nil {
		fail("missing const %s", name)
	}
	e.usable(src)
	if e.enum(src, found.Type) != want {
		fail("enum constant type mismatch: %s", name)
	}
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		fail("only literal string constants supported: %s", name)
	}
	// The standard type checker, not textual matching, validates string literals.
	tv, err := types.Eval(e.fs, nil, token.NoPos, lit.Value)
	if err != nil || tv.Value == nil || tv.Value.Kind() != constant.String {
		fail("bad string constant: %s", name)
	}
	value := constant.StringVal(tv.Value)
	out := make([]int, len(value))
	for i, b := range []byte(value) {
		out[i] = int(b)
	}
	return out
}
func (e *extractor) input(x ast.Expr) (int, string, bool) {
	switch n := x.(type) {
	case *ast.Ident:
		i, ok := e.inputs[n.Name]
		return i, e.enums[n.Name], ok
	case *ast.SelectorExpr:
		if id, ok := n.X.(*ast.Ident); ok {
			key := id.Name + "." + n.Sel.Name
			i, ok := e.inputs[key]
			return i, e.enums[key], ok
		}
	}
	return 0, "", false
}
func (e *extractor) expr(s source, x ast.Expr) *node {
	switch n := x.(type) {
	case *ast.ParenExpr:
		return e.expr(s, n.X)
	case *ast.Ident:
		if n.Name == "true" || n.Name == "false" {
			return &node{Kind: "bool", Value: n.Name == "true"}
		}
	case *ast.UnaryExpr:
		if n.Op == token.NOT {
			return &node{Kind: "not", Args: []*node{e.expr(s, n.X)}}
		}
	case *ast.BinaryExpr:
		if n.Op == token.LOR || n.Op == token.LAND {
			k := "or"
			if n.Op == token.LAND {
				k = "and"
			}
			return &node{Kind: k, Args: []*node{e.expr(s, n.X), e.expr(s, n.Y)}}
		}
		if n.Op == token.EQL || n.Op == token.NEQ {
			i, t, ok := e.input(n.X)
			rhs := n.Y
			if !ok {
				i, t, ok = e.input(n.Y)
				rhs = n.X
			}
			if !ok {
				fail("equality must compare an input and typed constant")
			}
			r := &node{Kind: "eq", Index: i, Bytes: e.literal(s, rhs, t)}
			if n.Op == token.NEQ {
				return &node{Kind: "not", Args: []*node{r}}
			}
			return r
		}
	}
	fail("unsupported expression %T", x)
	return nil
}
func (e *extractor) block(s source, ss []ast.Stmt) *node {
	if len(ss) == 0 {
		fail("non-total block")
	}
	head, tail := ss[0], ss[1:]
	switch n := head.(type) {
	case *ast.ReturnStmt:
		if len(tail) != 0 || len(n.Results) != 1 {
			fail("unsupported/unreachable return")
		}
		return e.expr(s, n.Results[0])
	case *ast.IfStmt:
		if n.Init != nil {
			fail("if init unsupported")
		}
		yes := e.block(s, n.Body.List)
		var no *node
		if n.Else == nil {
			no = e.block(s, tail)
		} else {
			if len(tail) != 0 {
				fail("unreachable trailing statements")
			}
			switch el := n.Else.(type) {
			case *ast.BlockStmt:
				no = e.block(s, el.List)
			case *ast.IfStmt:
				no = e.block(s, []ast.Stmt{el})
			default:
				fail("unsupported else")
			}
		}
		return &node{Kind: "if", Args: []*node{e.expr(s, n.Cond), yes, no}}
	case *ast.SwitchStmt:
		if n.Init != nil || len(tail) != 0 {
			fail("switch init/trailing statements unsupported")
		}
		i, t, ok := e.input(n.Tag)
		if !ok {
			fail("switch tag must be input")
		}
		var rest *node
		seen := map[string]bool{}
		for _, st := range n.Body.List {
			c := st.(*ast.CaseClause)
			if len(c.List) == 0 {
				if rest != nil {
					fail("duplicate default")
				}
				rest = e.block(s, c.Body)
			}
		}
		if rest == nil {
			fail("switch must have total default")
		}
		for j := len(n.Body.List) - 1; j >= 0; j-- {
			c := n.Body.List[j].(*ast.CaseClause)
			if len(c.List) == 0 {
				continue
			}
			var cond *node
			for k := len(c.List) - 1; k >= 0; k-- {
				bs := e.literal(s, c.List[k], t)
				key := fmt.Sprint(bs)
				if seen[key] {
					fail("duplicate switch constant")
				}
				seen[key] = true
				eq := &node{Kind: "eq", Index: i, Bytes: bs}
				if cond == nil {
					cond = eq
				} else {
					cond = &node{Kind: "or", Args: []*node{eq, cond}}
				}
			}
			rest = &node{Kind: "if", Args: []*node{cond, e.block(s, c.Body), rest}}
		}
		return rest
	}
	fail("unsupported statement %T", head)
	return nil
}
func (e *extractor) extract(t target) result {
	e.used = map[string]bool{}
	var s source
	var fn *ast.FuncDecl
	for _, f := range e.load(filepath.ToSlash(filepath.Dir(t.File))) {
		if f.path != t.File {
			continue
		}
		s = f
		for _, d := range f.file.Decls {
			n, ok := d.(*ast.FuncDecl)
			if !ok || n.Name.Name != t.Name {
				continue
			}
			if t.Receiver == "" {
				if n.Recv != nil {
					continue
				}
			} else {
				if n.Recv == nil || len(n.Recv.List) != 1 {
					continue
				}
				id, ok := n.Recv.List[0].Type.(*ast.Ident)
				if !ok || id.Name != t.Receiver {
					continue
				}
			}
			if fn != nil {
				fail("duplicate target")
			}
			fn = n
		}
	}
	if fn == nil {
		fail("missing function %s", t.ID)
	}
	e.usable(s)
	if fn.Type.TypeParams != nil || fn.Type.Results == nil || len(fn.Type.Results.List) != 1 || len(fn.Type.Results.List[0].Names) != 0 {
		fail("unsupported signature")
	}
	r, ok := fn.Type.Results.List[0].Type.(*ast.Ident)
	if !ok || r.Name != "bool" {
		fail("expected bool result")
	}
	e.inputs = map[string]int{}
	e.enums = map[string]string{}
	var inputs []string
	fields := fn.Type.Params.List
	if fn.Recv != nil {
		fields = append(append([]*ast.Field{}, fn.Recv.List...), fields...)
	}
	for _, f := range fields {
		if len(f.Names) != 1 {
			fail("one explicit name per input required")
		}
		name := f.Names[0].Name
		if name == "_" || name == "true" || name == "false" || name == "string" || name == "bool" {
			fail("unsupported input binding %s", name)
		}
		if _, exists := e.inputs[name]; exists {
			fail("duplicate input binding")
		}
		// Qualified package aliases cannot be shadowed by value parameters.
		for _, im := range s.file.Imports {
			p, _ := strconv.Unquote(im.Path.Value)
			alias := filepath.Base(p)
			if im.Name != nil {
				alias = im.Name.Name
			}
			if alias == name {
				fail("input shadows import %s", name)
			}
		}
		typ := f.Type
		if t.ID == "workload_terminal" {
			dir, tn := e.named(s, typ)
			if tn != "Operation" {
				fail("expected value Operation")
			}
			ts, td := e.typeDecl(dir, tn)
			st, ok := td.Type.(*ast.StructType)
			if !ok {
				fail("expected struct")
			}
			var status ast.Expr
			for _, field := range st.Fields.List {
				for _, n := range field.Names {
					if n.Name == "Status" {
						status = field.Type
					}
				}
			}
			if status == nil {
				fail("Operation.Status missing")
			}
			typ = status
			saved := s
			s = ts
			e.enums[name+".Status"] = e.enum(s, typ)
			s = saved
			name += ".Status"
		} else {
			e.enums[name] = e.enum(s, typ)
		}
		if len(inputs) >= 2 {
			fail("at most two inputs")
		}
		e.inputs[name] = len(inputs)
		inputs = append(inputs, name)
	}
	if len(inputs) == 0 {
		fail("no input")
	}
	body := e.block(s, fn.Body.List)
	dependencies := []string{}
	for path := range e.used {
		dependencies = append(dependencies, path)
	}
	sort.Strings(dependencies)
	return result{ID: t.ID, File: t.File, Name: t.Name, Receiver: t.Receiver, Inputs: inputs, AST: body, Dependencies: dependencies}
}
func main() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintln(os.Stderr, "restricted extraction rejected:", r)
			os.Exit(1)
		}
	}()
	root := "."
	if len(os.Args) == 2 {
		root = os.Args[1]
	} else if len(os.Args) != 1 {
		fail("usage: extract [root]")
	}
	e := extractor{root: root, fs: token.NewFileSet(), packages: map[string][]source{}}
	out := []result{}
	for _, t := range targets {
		out = append(out, e.extract(t))
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		panic(err)
	}
}
