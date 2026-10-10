package taskgraph

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
)

// Fingerprint identifies the content, names, types and modes of declared inputs.
// Nil claims cover the whole workspace; an explicit empty list covers nothing.
// The trusted workspace root is canonicalized, but symlinks below it are refused.
// Two complete observations must agree, including metadata used only to detect
// concurrent mutations. Errors never produce a fingerprint suitable for reuse.
func Fingerprint(workspace string, paths []string) (string, error) {
	claims, err := normalizeClaims(paths)
	if err != nil {
		return "", err
	}
	if claims == nil {
		claims = []string{"."}
	}
	absolute, err := filepath.Abs(workspace)
	if err != nil {
		return "", err
	}
	canonical, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", err
	}
	root, err := os.OpenRoot(canonical)
	if err != nil {
		return "", err
	}
	defer root.Close()
	first, err := fingerprintSnapshot(root, claims)
	if err != nil {
		return "", err
	}
	second, err := fingerprintSnapshot(root, claims)
	if err != nil {
		return "", err
	}
	if first.digest != second.digest || len(first.observed) != len(second.observed) {
		return "", errors.New("workspace changed while fingerprinting")
	}
	for name, before := range first.observed {
		after, exists := second.observed[name]
		if !exists || !fingerprintSameInfo(before, after) {
			return "", fmt.Errorf("workspace path %q changed while fingerprinting", name)
		}
	}
	return first.digest, nil
}

type fingerprintScan struct {
	root     *os.Root
	hash     hash.Hash
	observed map[string]os.FileInfo
	digest   string
}

func fingerprintSnapshot(root *os.Root, claims []string) (*fingerprintScan, error) {
	s := &fingerprintScan{root: root, hash: sha256.New(), observed: map[string]os.FileInfo{}}
	s.field("taskgraph-fingerprint-v1")
	for _, name := range claims {
		s.field("claim")
		s.field(name)
		if err := s.ancestors(name); err != nil {
			return nil, err
		}
		if err := s.visit(name); err != nil {
			return nil, err
		}
	}
	s.digest = hex.EncodeToString(s.hash.Sum(nil))
	return s, nil
}
func (s *fingerprintScan) field(value string) {
	var size [8]byte
	binary.BigEndian.PutUint64(size[:], uint64(len(value)))
	s.hash.Write(size[:])
	s.hash.Write([]byte(value))
}
func fingerprintSameInfo(a, b os.FileInfo) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return os.SameFile(a, b) && a.Mode() == b.Mode() && a.Size() == b.Size() && a.ModTime().Equal(b.ModTime())
}
func (s *fingerprintScan) observe(name string, info os.FileInfo) error {
	if old, exists := s.observed[name]; exists && !fingerprintSameInfo(old, info) {
		return fmt.Errorf("workspace path %q changed while fingerprinting", name)
	}
	s.observed[name] = info
	return nil
}
func (s *fingerprintScan) stat(name string) (os.FileInfo, error) {
	info, err := s.root.Lstat(name)
	if errors.Is(err, os.ErrNotExist) {
		if err := s.observe(name, nil); err != nil {
			return nil, err
		}
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("symlink workspace path %q is unsupported", name)
	}
	if !info.IsDir() && !info.Mode().IsRegular() {
		return nil, fmt.Errorf("unsupported workspace file type %q", name)
	}
	if err := s.observe(name, info); err != nil {
		return nil, err
	}
	return info, nil
}
func (s *fingerprintScan) ancestors(name string) error {
	if _, err := s.stat("."); err != nil {
		return err
	}
	parts := strings.Split(name, "/")
	for i := 1; i < len(parts); i++ {
		prefix := strings.Join(parts[:i], "/")
		info, err := s.stat(prefix)
		if err != nil {
			return err
		}
		if info == nil {
			return nil
		}
		if !info.IsDir() {
			return fmt.Errorf("workspace ancestor %q is not a directory", prefix)
		}
	}
	return nil
}
func (s *fingerprintScan) visit(name string) error {
	info, err := s.stat(name)
	if err != nil {
		return err
	}
	s.field(name)
	if info == nil {
		s.field("missing")
		return nil
	}
	s.field(info.Mode().String())
	// O_NOFOLLOW rejects replacement of the final component by a symlink.
	// os.Root confines all component resolution even if ancestors change.
	file, err := s.root.OpenFile(name, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return fmt.Errorf("open workspace path %q: %w", name, err)
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil {
		return err
	}
	if !fingerprintSameInfo(info, opened) {
		return fmt.Errorf("workspace path %q changed before reading", name)
	}
	if info.IsDir() {
		s.field("directory")
		entries, err := file.ReadDir(-1)
		if err != nil {
			return err
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
		for _, entry := range entries {
			if strings.EqualFold(entry.Name(), ".git") {
				continue
			}
			if err := s.visit(path.Join(name, entry.Name())); err != nil {
				return err
			}
		}
	} else {
		s.field("file")
		var size [8]byte
		binary.BigEndian.PutUint64(size[:], uint64(info.Size()))
		s.hash.Write(size[:])
		n, err := io.Copy(s.hash, file)
		if err != nil {
			return err
		}
		if n != info.Size() {
			return fmt.Errorf("workspace path %q changed size while reading", name)
		}
	}
	after, err := file.Stat()
	if err != nil {
		return err
	}
	if !fingerprintSameInfo(info, after) {
		return fmt.Errorf("workspace path %q changed while reading", name)
	}
	current, err := s.stat(name)
	if err != nil {
		return err
	}
	if !fingerprintSameInfo(info, current) {
		return fmt.Errorf("workspace path %q changed after reading", name)
	}
	return s.ancestors(name)
}
