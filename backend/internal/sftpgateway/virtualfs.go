package sftpgateway

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

var (
	ErrPathEscape   = errors.New("path escapes virtual root")
	ErrRenameDenied = errors.New("rename into production paths is not allowed")
)

// VirtualFS confines SFTP paths to a single OS directory (virtual root).
type VirtualFS struct {
	root string
}

// NewVirtualFS creates a virtual FS rooted at root (must exist).
func NewVirtualFS(root string) (*VirtualFS, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("virtual root: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("virtual root: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("virtual root is not a directory: %s", abs)
	}
	return &VirtualFS{root: abs}, nil
}

// Root returns the absolute virtual root path.
func (v *VirtualFS) Root() string {
	return v.root
}

func (v *VirtualFS) resolve(name string) (string, error) {
	if v == nil || v.root == "" {
		return "", fmt.Errorf("virtual fs: %w", ErrPathEscape)
	}
	clean := cleanSFTPPath(name)
	if clean == "" || clean == "." {
		return v.root, nil
	}
	joined := filepath.Join(v.root, filepath.FromSlash(clean))
	abs, err := filepath.Abs(joined)
	if err != nil {
		return "", err
	}
	if !withinRoot(v.root, abs) {
		return "", ErrPathEscape
	}
	return abs, nil
}

func cleanSFTPPath(name string) string {
	name = strings.TrimSpace(name)
	if name == "" || name == "." {
		return ""
	}
	name = filepath.ToSlash(name)
	name = strings.TrimPrefix(name, "/")
	clean := pathCleanSegments(name)
	if clean == "." || clean == "" {
		return ""
	}
	return clean
}

func pathCleanSegments(name string) string {
	parts := strings.Split(name, "/")
	var out []string
	for _, p := range parts {
		switch p {
		case "", ".":
			continue
		case "..":
			if len(out) > 0 {
				out = out[:len(out)-1]
			} else {
				return ".."
			}
		default:
			out = append(out, p)
		}
	}
	return strings.Join(out, "/")
}

func pathsOverlap(a, b string) bool {
	if strings.TrimSpace(a) == "" || strings.TrimSpace(b) == "" {
		return false
	}
	aa, err := filepath.Abs(a)
	if err != nil {
		return false
	}
	bb, err := filepath.Abs(b)
	if err != nil {
		return false
	}
	return withinRoot(aa, bb) || withinRoot(bb, aa)
}

func withinRoot(root, target string) bool {
	root = filepath.Clean(root)
	target = filepath.Clean(target)
	if root == target {
		return true
	}
	sep := string(os.PathSeparator)
	return strings.HasPrefix(target, root+sep)
}

// Open opens a file relative to the virtual root.
func (v *VirtualFS) Open(name string) (*os.File, error) {
	path, err := v.resolve(name)
	if err != nil {
		return nil, err
	}
	return os.Open(path)
}

// OpenFile opens or creates a file relative to the virtual root.
func (v *VirtualFS) OpenFile(name string, flag int, perm os.FileMode) (*os.File, error) {
	path, err := v.resolve(name)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	return os.OpenFile(path, flag, perm)
}

// Stat returns file info for a virtual path.
func (v *VirtualFS) Stat(name string) (os.FileInfo, error) {
	path, err := v.resolve(name)
	if err != nil {
		return nil, err
	}
	return os.Stat(path)
}

// Mkdir creates a directory relative to the virtual root.
func (v *VirtualFS) Mkdir(name string) error {
	path, err := v.resolve(name)
	if err != nil {
		return err
	}
	return os.Mkdir(path, 0o755)
}

// Remove removes a file or empty directory.
func (v *VirtualFS) Remove(name string) error {
	path, err := v.resolve(name)
	if err != nil {
		return err
	}
	return os.Remove(path)
}

type listDir struct {
	entries []os.FileInfo
	offset  int
}

func (l *listDir) ListAt(f []fs.FileInfo, offset int64) (int, error) {
	if offset >= int64(len(l.entries)) {
		return 0, io.EOF
	}
	n := copy(f, l.entries[offset:])
	if int(offset)+n >= len(l.entries) {
		return n, io.EOF
	}
	return n, nil
}

// List returns directory entries for a virtual path.
func (v *VirtualFS) List(name string) (fs.FileInfo, []os.FileInfo, error) {
	path, err := v.resolve(name)
	if err != nil {
		return nil, nil, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, nil, err
	}
	if !info.IsDir() {
		return nil, nil, fmt.Errorf("not a directory")
	}
	ents, err := os.ReadDir(path)
	if err != nil {
		return nil, nil, err
	}
	out := make([]os.FileInfo, 0, len(ents))
	for _, e := range ents {
		fi, err := e.Info()
		if err != nil {
			return nil, nil, err
		}
		out = append(out, fi)
	}
	return info, out, nil
}
