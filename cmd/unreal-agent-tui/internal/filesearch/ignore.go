package filesearch

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	gitignore "github.com/denormal/go-gitignore"
)

type ignoreRules struct {
	parent *ignoreRules
	base   string
	rules  gitignore.GitIgnore
}

func (rules *ignoreRules) ignored(path string, directory bool) bool {
	for current := rules; current != nil; current = current.parent {
		relative, err := filepath.Rel(current.base, path)
		if err != nil {
			continue
		}
		if match := current.rules.Relative(filepath.ToSlash(relative), directory); match != nil {
			return match.Ignore()
		}
	}
	return false
}

func loadIgnoreRules(parent *ignoreRules, directory string) (*ignoreRules, error) {
	path := filepath.Join(directory, ".gitignore")
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return parent, nil
	}
	if err != nil {
		return parent, err
	}
	if !info.Mode().IsRegular() {
		return parent, nil
	}
	file, err := os.Open(path)
	if err != nil {
		return parent, err
	}
	defer func() { _ = file.Close() }()
	const limit = 1 << 20
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return parent, err
	}
	if len(data) > limit {
		return parent, fmt.Errorf("%s exceeds %d bytes", path, limit)
	}
	var parseError error
	parsed := gitignore.New(strings.NewReader(string(data)), directory, func(err gitignore.Error) bool {
		if parseError == nil {
			parseError = err
		}
		return true
	})
	return &ignoreRules{parent: parent, base: directory, rules: parsed}, parseError
}
