// Command inventory inventories source obligations; it does not verify Go.
// It parses every tracked/new nonignored production .go file, including all
// build variants. The output is data, never a proof certificate.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

type Function struct {
	ID         string `json:"id"`
	File       string `json:"file"`
	Name       string `json:"name"`
	Receiver   string `json:"receiver,omitempty"`
	Line       int    `json:"line"`
	BodySHA256 string `json:"body_sha256"`
	Body       bool   `json:"has_body"`
}

type Source struct {
	Path        string     `json:"path"`
	Package     string     `json:"package"`
	SHA256      string     `json:"sha256"`
	Constraints []string   `json:"build_constraints,omitempty"`
	Functions   []Function `json:"functions"`
}

type Inventory struct {
	Schema    int      `json:"schema"`
	Toolchain string   `json:"toolchain"`
	Sources   []Source `json:"sources"`
}

func hash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func inspect(root string) (Inventory, error) {
	command := exec.Command("git", "-C", root, "ls-files", "--cached", "--others", "--exclude-standard", "-z")
	data, err := command.Output()
	if err != nil {
		return Inventory{}, fmt.Errorf("source inventory: %w", err)
	}
	unique := map[string]bool{}
	for _, name := range strings.Split(string(data), "\x00") {
		if strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go") {
			unique[name] = true
		}
	}
	names := make([]string, 0, len(unique))
	for name := range unique {
		names = append(names, name)
	}
	sort.Strings(names)
	result := Inventory{Schema: 1, Toolchain: runtime.Version(), Sources: []Source{}}
	for _, name := range names {
		path := filepath.Join(root, filepath.FromSlash(name))
		real, err := filepath.EvalSymlinks(path)
		if err != nil {
			return Inventory{}, err
		}
		if real != path {
			return Inventory{}, fmt.Errorf("source symlink not allowed: %s", name)
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return Inventory{}, err
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, contents, parser.ParseComments|parser.AllErrors)
		if err != nil {
			return Inventory{}, fmt.Errorf("parse %s: %w", name, err)
		}
		source := Source{Path: name, Package: file.Name.Name, SHA256: hash(contents), Functions: []Function{}}
		for _, group := range file.Comments {
			if group.Pos() > file.Package {
				break
			}
			for _, comment := range group.List {
				if strings.HasPrefix(comment.Text, "//go:build ") || strings.HasPrefix(comment.Text, "// +build ") {
					source.Constraints = append(source.Constraints, comment.Text)
				}
			}
		}
		seen := map[string]int{}
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok {
				continue
			}
			receiver := ""
			if function.Recv != nil {
				if len(function.Recv.List) != 1 {
					return Inventory{}, fmt.Errorf("invalid receiver: %s", name)
				}
				var output bytes.Buffer
				if err := printer.Fprint(&output, fset, function.Recv.List[0].Type); err != nil {
					return Inventory{}, err
				}
				receiver = output.String()
			}
			symbol := function.Name.Name
			if receiver != "" {
				symbol = "(" + receiver + ")." + symbol
			}
			seen[symbol]++
			id := name + "::" + symbol
			if seen[symbol] > 1 {
				id += fmt.Sprintf("#%d", seen[symbol])
			}
			item := Function{ID: id, File: name, Name: function.Name.Name, Receiver: receiver,
				Line: fset.Position(function.Pos()).Line, Body: function.Body != nil}
			if function.Body != nil {
				start := fset.Position(function.Body.Pos()).Offset
				end := fset.Position(function.Body.End()).Offset
				item.BodySHA256 = hash(contents[start:end])
			}
			source.Functions = append(source.Functions, item)
		}
		result.Sources = append(result.Sources, source)
	}
	return result, nil
}

func main() {
	root := flag.String("root", ".", "repository root")
	flag.Parse()
	absolute, err := filepath.Abs(*root)
	if err == nil {
		absolute, err = filepath.EvalSymlinks(absolute)
	}
	var inventory Inventory
	if err == nil {
		inventory, err = inspect(absolute)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "inventory:", err)
		os.Exit(1)
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(inventory); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
