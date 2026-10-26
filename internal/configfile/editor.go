// Package configfile edits one module-selected text file, never a client-selected path.
package configfile

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"unicode/utf8"

	"github.com/zelport/zelport/internal/privatefile"
)

const MaxSize = 1 << 20

var (
	ErrConflict    = errors.New("The file changed. Copy your draft or reload the file.")
	ErrUnavailable = errors.New("The configuration file could not be read or written.")
	ErrInvalid     = errors.New("Use a UTF-8 text file without NUL characters, at most 1 MiB.")
)

type Document struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Version string `json:"version"`
}
type snapshot struct {
	path, target string
	binding      [32]byte
	info, link   os.FileInfo
	hash         [32]byte
}

// Editor retains only bounded file metadata and digests, never file contents.
// The zero value is ready to use. Versions are random, process-local capabilities.
type Editor struct {
	mu       sync.Mutex
	versions map[string]snapshot
	order    []string
}

func valid(data []byte) bool {
	return len(data) <= MaxSize && utf8.Valid(data) && !bytes.ContainsRune(data, 0)
}

// resolve follows existing parent links but rejects dangling links, including
// dangling ancestors. A missing leaf/parent is valid until explicit Save.
func resolve(path string) (string, error) {
	info, err := os.Lstat(path)
	if err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return filepath.EvalSymlinks(path)
		}
		return filepath.EvalSymlinks(path)
	}
	if !os.IsNotExist(err) {
		return "", ErrUnavailable
	}
	parent := filepath.Dir(path)
	if parent == path {
		return "", ErrUnavailable
	}
	resolved, err := resolve(parent)
	if err != nil {
		return "", err
	}
	return filepath.Join(resolved, filepath.Base(path)), nil
}
func inspect(path, binding string) (snapshot, []byte, error) {
	s := snapshot{path: path, binding: sha256.Sum256([]byte(binding))}
	if !filepath.IsAbs(path) {
		return s, nil, ErrUnavailable
	}
	var err error
	s.target, err = resolve(path)
	if err != nil {
		return s, nil, ErrUnavailable
	}
	s.link, err = os.Lstat(path)
	if err != nil && !os.IsNotExist(err) {
		return s, nil, ErrUnavailable
	}
	s.info, err = os.Stat(s.target)
	if os.IsNotExist(err) {
		return s, []byte{}, nil
	}
	if err != nil || !s.info.Mode().IsRegular() || s.info.Size() > MaxSize {
		return s, nil, ErrUnavailable
	}
	file, err := openRead(s.target)
	if err != nil {
		return s, nil, ErrUnavailable
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || !os.SameFile(s.info, info) {
		return s, nil, ErrConflict
	}
	data, err := io.ReadAll(io.LimitReader(file, MaxSize+1))
	if err != nil {
		return s, nil, ErrUnavailable
	}
	if !valid(data) {
		return s, nil, ErrInvalid
	}
	after, err := file.Stat()
	if err != nil || !sameInfo(info, after) {
		return s, nil, ErrConflict
	}
	s.info = after
	s.hash = sha256.Sum256(data)
	return s, data, nil
}
func sameInfo(a, b os.FileInfo) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return os.SameFile(a, b) && a.Mode() == b.Mode() && a.Size() == b.Size() && a.ModTime().Equal(b.ModTime())
}
func same(a, b snapshot) bool {
	return a.path == b.path && a.target == b.target && a.binding == b.binding && a.hash == b.hash && sameInfo(a.info, b.info) && sameInfo(a.link, b.link)
}
func (e *Editor) remember(s snapshot, data []byte) (Document, error) {
	var nonce [32]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return Document{}, ErrUnavailable
	}
	token := hex.EncodeToString(nonce[:])
	if e.versions == nil {
		e.versions = map[string]snapshot{}
	}
	if len(e.order) >= 128 {
		delete(e.versions, e.order[0])
		e.order = e.order[1:]
	}
	e.order = append(e.order, token)
	e.versions[token] = s
	return Document{Path: s.path, Content: string(data), Version: token}, nil
}
func (e *Editor) Read(path, binding string) (Document, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	s, data, err := inspect(path, binding)
	if err != nil {
		return Document{}, err
	}
	return e.remember(s, data)
}
func (e *Editor) Write(ctx context.Context, path, binding, version, content string, validate func([]byte) error) (Document, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if ctx.Err() != nil {
		return Document{}, ErrUnavailable
	}
	old, ok := e.versions[version]
	if !ok || old.path != path || old.binding != sha256.Sum256([]byte(binding)) {
		return Document{}, ErrConflict
	}
	data := []byte(content)
	if !valid(data) {
		return Document{}, ErrInvalid
	}
	current, _, err := inspect(path, binding)
	if err != nil || !same(old, current) {
		return Document{}, ErrConflict
	}
	if validate != nil {
		if err = validate(data); err != nil {
			return Document{}, err
		}
	}
	if ctx.Err() != nil {
		return Document{}, ErrUnavailable
	}
	if err = os.MkdirAll(filepath.Dir(current.target), 0700); err != nil {
		return Document{}, ErrUnavailable
	}
	file, err := privatefile.CreateTemp(filepath.Dir(current.target))
	if err != nil {
		return Document{}, ErrUnavailable
	}
	defer os.Remove(file.Name())
	defer file.Close()
	if _, err = file.Write(data); err != nil {
		return Document{}, ErrUnavailable
	}
	if current.info != nil {
		if err = file.Chmod(current.info.Mode().Perm()); err != nil {
			return Document{}, ErrUnavailable
		}
	}
	if err = file.Sync(); err != nil {
		return Document{}, ErrUnavailable
	}
	if err = file.Close(); err != nil {
		return Document{}, ErrUnavailable
	}
	latest, _, err := inspect(path, binding)
	if err != nil || !same(current, latest) {
		return Document{}, ErrConflict
	}
	if ctx.Err() != nil {
		return Document{}, ErrUnavailable
	}
	if current.info == nil {
		err = os.Link(file.Name(), current.target)
	} else {
		err = privatefile.Replace(file.Name(), current.target)
	}
	if os.IsExist(err) {
		return Document{}, ErrConflict
	}
	if err != nil {
		return Document{}, ErrUnavailable
	}
	delete(e.versions, version)
	// Do not perform a fallible re-read after publishing: the caller must be able
	// to distinguish a successful save from a failed write. Reload gets a new version.
	return Document{Path: path, Content: content}, nil
}
