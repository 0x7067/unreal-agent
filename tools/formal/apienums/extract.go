// Command extract is a fail-closed Go-AST frontend for generated enum Valid.
// The parser, type checker, and this deliberately restricted translation are TCB.
package main

import (
	"crypto/sha256"
	"encoding/hex"
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

type node struct {
	Kind  string  `json:"kind"`
	Value bool    `json:"value,omitempty"`
	Bytes []int   `json:"bytes,omitempty"`
	Args  []*node `json:"args,omitempty"`
}
type declaration struct {
	Name  string `json:"name"`
	Bytes []int  `json:"bytes"`
}
type policy struct {
	ID        string        `json:"id"`
	File      string        `json:"file"`
	Receiver  string        `json:"receiver"`
	Constants []declaration `json:"constants"`
}
type dependency struct {
	File string `json:"file"`
	SHA  string `json:"source_sha256"`
}
type record struct {
	ID             string       `json:"id"`
	File           string       `json:"file"`
	Name           string       `json:"name"`
	Receiver       string       `json:"receiver"`
	SHA            string       `json:"source_sha256"`
	Law            string       `json:"law"`
	Classification string       `json:"classification"`
	AST            *node        `json:"ast"`
	Dependencies   []dependency `json:"dependencies"`
}
type output struct {
	Records []record `json:"records"`
	Policy  []policy `json:"policy"`
}

func fail(f string, args ...any) { panic(fmt.Sprintf(f, args...)) }
func safe(root, rel string) string {
	p := filepath.Join(root, rel)
	actual, err := filepath.EvalSymlinks(p)
	if err != nil {
		panic(err)
	}
	if actual != p {
		fail("symlink/escape not admitted: %s", p)
	}
	return p
}
func boolNode(v bool) *node { return &node{Kind: "bool", Value: v} }
func expr(x ast.Expr, receiver string, want types.Type, info *types.Info) *node {
	switch n := x.(type) {
	case *ast.ParenExpr:
		return expr(n.X, receiver, want, info)
	case *ast.Ident:
		o := info.Uses[n]
		c, ok := o.(*types.Const)
		if !ok || c.Parent() != types.Universe || c.Val().Kind() != constant.Bool {
			fail("return must use predeclared true/false, got %s", n.Name)
		}
		return boolNode(constant.BoolVal(c.Val()))
	default:
		fail("unsupported return expression %T", x)
	}
	return nil
}
func returned(stmts []ast.Stmt, receiver string, want types.Type, info *types.Info) *node {
	if len(stmts) != 1 {
		fail("case must contain exactly one return (no assignment, fallthrough, calls)")
	}
	r, ok := stmts[0].(*ast.ReturnStmt)
	if !ok || len(r.Results) != 1 {
		fail("case must contain one single-value return")
	}
	return expr(r.Results[0], receiver, want, info)
}
func method(fn *ast.FuncDecl, info *types.Info) (string, types.Type, string) {
	if fn.Recv == nil || len(fn.Recv.List) != 1 || len(fn.Recv.List[0].Names) != 1 {
		fail("expected one named value receiver")
	}
	field := fn.Recv.List[0]
	id, ok := field.Type.(*ast.Ident)
	if !ok {
		fail("pointer/generic/non-name receiver unsupported")
	}
	recv := info.Defs[field.Names[0]]
	if recv == nil {
		fail("missing checked receiver")
	}
	typ := recv.Type()
	named, ok := typ.(*types.Named)
	if !ok || named.Obj().Name() != id.Name {
		fail("receiver must be a defined named type")
	}
	basic, ok := typ.Underlying().(*types.Basic)
	if !ok || basic.Kind() != types.String {
		fail("receiver %s not string", id.Name)
	}
	if fn.Type.TypeParams != nil || (fn.Type.Params != nil && len(fn.Type.Params.List) != 0) || fn.Type.Results == nil || len(fn.Type.Results.List) != 1 {
		fail("Valid signature must be () bool")
	}
	ret := fn.Type.Results.List[0]
	retID, ok := ret.Type.(*ast.Ident)
	if !ok || retID.Name != "bool" || info.Uses[retID] != types.Universe.Lookup("bool") || len(ret.Names) != 0 {
		fail("Valid requires unnamed predeclared bool result")
	}
	return field.Names[0].Name, typ, id.Name
}
func body(fn *ast.FuncDecl, recv string, typ types.Type, info *types.Info) *node {
	if fn.Body == nil || len(fn.Body.List) != 1 {
		fail("Valid body must be exactly one switch")
	}
	sw, ok := fn.Body.List[0].(*ast.SwitchStmt)
	if !ok || sw.Init != nil || sw.Tag == nil {
		fail("expected tagged switch without init")
	}
	tag, ok := sw.Tag.(*ast.Ident)
	if !ok || tag.Name != recv || info.Uses[tag] != info.Defs[fn.Recv.List[0].Names[0]] {
		fail("switch tag must be the exact receiver binding")
	}
	var guards []*node
	var results []*node
	var fallback *node
	for _, stmt := range sw.Body.List {
		c, ok := stmt.(*ast.CaseClause)
		if !ok {
			fail("unsupported switch clause %T", stmt)
		}
		result := returned(c.Body, recv, typ, info)
		if c.List == nil {
			if fallback != nil {
				fail("multiple default cases")
			}
			fallback = result
			continue
		}
		var guard *node
		for _, test := range c.List {
			id, ok := test.(*ast.Ident)
			if !ok {
				fail("case requires declared enum constant, no calls/literals/expressions")
			}
			con, ok := info.Uses[id].(*types.Const)
			if !ok || !types.Identical(con.Type(), typ) || con.Val().Kind() != constant.String {
				fail("case constant %s not declared at receiver type", id.Name)
			}
			s := constant.StringVal(con.Val())
			bytes := make([]int, len(s))
			for i, b := range []byte(s) {
				bytes[i] = int(b)
			}
			eq := &node{Kind: "eq", Bytes: bytes}
			if guard == nil {
				guard = eq
			} else {
				guard = &node{Kind: "or", Args: []*node{guard, eq}}
			}
		}
		guards = append(guards, guard)
		results = append(results, result)
	}
	if fallback == nil {
		fail("switch must have explicit default return")
	}
	for i := len(guards) - 1; i >= 0; i-- {
		fallback = &node{Kind: "if", Args: []*node{guards[i], results[i], fallback}}
	}
	return fallback
}
func extract(root, pkg string) ([]record, []policy) {
	dir := safe(root, "internal/"+pkg)
	entries, err := os.ReadDir(dir)
	if err != nil {
		panic(err)
	}
	fs := token.NewFileSet()
	var methods []*ast.FuncDecl
	paths := map[*ast.FuncDecl]string{}
	typeSpecs := map[string]*ast.TypeSpec{}
	var consts []ast.Decl
	var allTypes []*ast.TypeSpec
	var dependencies []dependency
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		rel := "internal/" + pkg + "/" + entry.Name()
		path := safe(root, rel)
		content, err := os.ReadFile(path)
		if err != nil {
			panic(err)
		}
		sum := sha256.Sum256(content)
		dependencies = append(dependencies, dependency{File: rel, SHA: hex.EncodeToString(sum[:])})
		f, err := parser.ParseFile(fs, path, content, parser.ParseComments|parser.AllErrors)
		if err != nil {
			panic(err)
		}
		if f.Name.Name != pkg {
			fail("unexpected package in %s", rel)
		}
		// Import bindings are file scoped but can also hide the predeclared types
		// and literals on which this restricted bridge depends. Dot imports have
		// unbounded unqualified bindings, so never admit them.
		for _, im := range f.Imports {
			path, err := strconv.Unquote(im.Path.Value)
			if err != nil {
				panic(err)
			}
			name := filepath.Base(path)
			if im.Name != nil {
				name = im.Name.Name
			}
			if name == "." || name == "bool" || name == "string" || name == "true" || name == "false" || path == "C" {
				fail("unsupported import binding %s in %s", name, rel)
			}
		}
		for _, g := range f.Comments {
			for _, c := range g.List {
				if strings.HasPrefix(c.Text, "//go:build") || strings.HasPrefix(c.Text, "// +build") {
					fail("build constraints unsupported in %s", rel)
				}
				if strings.HasPrefix(c.Text, "//go:") && strings.Fields(c.Text)[0] != "//go:generate" {
					fail("compiler directives unsupported in %s: %s", rel, c.Text)
				}
			}
		}
		for _, d := range f.Decls {
			switch n := d.(type) {
			case *ast.FuncDecl:
				if n.Name.Name == "Valid" {
					if entry.Name() != "types.gen.go" {
						fail("Valid outside inventoried generated source: %s", rel)
					}
					methods = append(methods, n)
					paths[n] = rel
				}
				if n.Recv == nil && (n.Name.Name == "bool" || n.Name.Name == "string" || n.Name.Name == "true" || n.Name.Name == "false") {
					fail("predeclared name shadowed")
				}
			case *ast.GenDecl:
				if n.Tok == token.CONST {
					consts = append(consts, n)
				}
				for _, sp := range n.Specs {
					switch s := sp.(type) {
					case *ast.TypeSpec:
						if typeSpecs[s.Name.Name] != nil {
							fail("duplicate type %s", s.Name.Name)
						}
						typeSpecs[s.Name.Name] = s
						allTypes = append(allTypes, s)
						if s.Name.Name == "bool" || s.Name.Name == "string" || s.Name.Name == "true" || s.Name.Name == "false" {
							fail("predeclared type shadowed")
						}
					case *ast.ValueSpec:
						for _, id := range s.Names {
							if id.Name == "bool" || id.Name == "string" || id.Name == "true" || id.Name == "false" {
								fail("predeclared value shadowed")
							}
						}
					}
				}
			}
		}
	}
	// Check all constants together with exactly the named string receiver declarations.
	// go/types resolves values, aliases, implicit const specs and Go string escapes;
	// unresolved/nonconstant expressions and duplicate switch values fail closed.
	selected := map[string]bool{}
	synthetic := &ast.File{Name: ast.NewIdent(pkg)}
	for _, fn := range methods {
		if fn.Recv == nil || len(fn.Recv.List) != 1 {
			fail("Valid must be method")
		}
		id, ok := fn.Recv.List[0].Type.(*ast.Ident)
		if !ok {
			fail("pointer/generic receiver unsupported")
		}
		t := typeSpecs[id.Name]
		if t == nil {
			fail("missing receiver type %s", id.Name)
		}
		base, ok := t.Type.(*ast.Ident)
		if !ok || base.Name != "string" || t.Assign.IsValid() || t.TypeParams != nil {
			fail("receiver %s must be defined directly over string, no aliases", id.Name)
		}
		selected[id.Name] = true
	}
	for _, t := range allTypes {
		if selected[t.Name.Name] {
			synthetic.Decls = append(synthetic.Decls, &ast.GenDecl{Tok: token.TYPE, Specs: []ast.Spec{t}})
		}
	}
	synthetic.Decls = append(synthetic.Decls, consts...)
	for _, fn := range methods {
		synthetic.Decls = append(synthetic.Decls, fn)
	}
	info := &types.Info{Defs: map[*ast.Ident]types.Object{}, Uses: map[*ast.Ident]types.Object{}}
	config := types.Config{}
	_, err = config.Check("restricted/"+pkg, fs, []*ast.File{synthetic}, info)
	if err != nil {
		fail("restricted type/constant validation: %v", err)
	}
	var records []record
	var policies []policy
	for _, fn := range methods {
		recv, typ, name := method(fn, info)
		id := pkg + "_" + name + "_Valid"
		rel := paths[fn]
		content, err := os.ReadFile(safe(root, rel))
		if err != nil {
			panic(err)
		}
		sum := sha256.Sum256(content)
		r := record{ID: id, File: rel, Name: "Valid", Receiver: name, SHA: hex.EncodeToString(sum[:]), Law: id + "_matches_policy", Classification: "source-linked-restricted-semantics", AST: body(fn, recv, typ, info), Dependencies: dependencies}
		p := policy{ID: id, File: rel, Receiver: name, Constants: []declaration{}}
		// Independent policy inventory: only declarations, never the method body.
		for ident, obj := range info.Defs {
			c, ok := obj.(*types.Const)
			if !ok || !types.Identical(c.Type(), typ) {
				continue
			}
			if c.Val().Kind() != constant.String {
				fail("nonstring enum constant")
			}
			s := constant.StringVal(c.Val())
			bytes := make([]int, len(s))
			for i, b := range []byte(s) {
				bytes[i] = int(b)
			}
			p.Constants = append(p.Constants, declaration{Name: ident.Name, Bytes: bytes})
		}
		// Canonical declared-name order, independent of map iteration and method bodies.
		sort.Slice(p.Constants, func(i, j int) bool { return p.Constants[i].Name < p.Constants[j].Name })
		if len(p.Constants) == 0 {
			fail("enum %s has no declarations", name)
		}
		records = append(records, r)
		policies = append(policies, p)
	}
	return records, policies
}
func main() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintln(os.Stderr, "apienums extract:", r)
			os.Exit(1)
		}
	}()
	if len(os.Args) != 2 {
		fail("usage: extract ROOT")
	}
	root, err := filepath.Abs(os.Args[1])
	if err != nil {
		panic(err)
	}
	out := output{Records: []record{}, Policy: []policy{}}
	for _, pkg := range []string{"openaiapi", "anthropicapi"} {
		r, p := extract(root, pkg)
		out.Records = append(out.Records, r...)
		out.Policy = append(out.Policy, p...)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		panic(err)
	}
}
