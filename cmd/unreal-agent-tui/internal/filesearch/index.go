package filesearch

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"
)

type Index struct{ files []string }

func NewIndex(paths []string) Index {
	index := Index{files: make([]string, 0, len(paths))}
	for _, path := range paths {
		index.add(path)
	}
	return index
}

func (index *Index) add(path string) {
	if !utf8.ValidString(path) || strings.ContainsFunc(path, unicode.IsControl) {
		return
	}
	index.files = append(index.files, path)
}

func Scan(ctx context.Context, root string) (Index, error) {
	var index Index
	var firstError error
	ignores := make(map[string]*ignoreRules)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			if path == root {
				return err
			}
			if firstError == nil {
				firstError = err
			}
			return nil
		}
		switch entry.Name() {
		case ".git", ".hg", ".svn":
			if entry.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		parentRules := ignores[filepath.Dir(path)]
		if path != root && parentRules.ignored(path, entry.IsDir()) {
			if entry.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			rules, err := loadIgnoreRules(parentRules, path)
			if err != nil && firstError == nil {
				firstError = err
			}
			ignores[path] = rules
			return nil
		}
		info, err := entry.Info()
		if err == nil && info.Mode()&fs.ModeSymlink != 0 {
			info, err = os.Stat(path)
		}
		if err != nil {
			if !os.IsNotExist(err) && firstError == nil {
				firstError = err
			}
			return nil
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		index.add(filepath.ToSlash(relative))
		return nil
	})
	if err != nil {
		return Index{}, err
	}
	return index, firstError
}
