package install

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"net/url"
	"path"
	"strings"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/domain/manifest"
)

var rejectedExts = map[string]struct{}{
	".sh":  {},
	".exe": {},
	".bat": {},
	".cmd": {},
	".ps1": {},
	".dll": {},
	".so":  {},
}

type InspectedZip struct {
	Manifest *manifest.Manifest
	Files    map[string][]byte
}

func InspectZip(raw []byte) (*InspectedZip, error) {
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return nil, err
	}

	files := make(map[string][]byte)
	var foundManifest *manifest.Manifest
	var topLevelID string
	registerFound := false

	for _, f := range zr.File {
		name, isDir, err := cleanZipName(f.Name)
		if err != nil {
			return nil, err
		}
		if isDir {
			continue
		}
		if _, ok := rejectedExts[strings.ToLower(path.Ext(name))]; ok {
			return nil, fmt.Errorf("plugin zip contains rejected extension: %s", name)
		}

		parts := strings.Split(name, "/")
		if len(parts) < 2 {
			return nil, fmt.Errorf("plugin zip entry must be under top-level directory: %s", name)
		}
		if topLevelID == "" {
			topLevelID = parts[0]
		} else if parts[0] != topLevelID {
			return nil, fmt.Errorf("plugin zip contains multiple top-level directories")
		}

		content, err := readZipFile(f)
		if err != nil {
			return nil, err
		}

		switch {
		case name == topLevelID+"/plugin.yaml" || name == topLevelID+"/backend/plugin.yaml":
			m, err := manifest.Parse(content)
			if err != nil {
				return nil, err
			}
			if m.ID != topLevelID {
				return nil, fmt.Errorf("plugin manifest id %q does not match top-level directory %q", m.ID, topLevelID)
			}
			foundManifest = m
		case strings.HasPrefix(name, topLevelID+"/backend/"):
			rel := strings.TrimPrefix(name, topLevelID+"/backend/")
			files[rel] = content
			if path.Base(rel) == "plugin.go" && FileDeclaresRegisterEngine(content) {
				registerFound = true
			}
		case strings.HasPrefix(name, topLevelID+"/frontend/"):
			rel := strings.TrimPrefix(name, topLevelID+"/frontend/")
			files[rel] = content
		}
	}

	if foundManifest == nil {
		return nil, fmt.Errorf("plugin manifest not found")
	}
	if !registerFound {
		return nil, fmt.Errorf("backend plugin.go with Register(*gin.Engine) not found")
	}

	return &InspectedZip{
		Manifest: foundManifest,
		Files:    files,
	}, nil
}

func cleanZipName(raw string) (string, bool, error) {
	if raw == "" || strings.Contains(raw, `\`) {
		return "", false, fmt.Errorf("unsafe plugin zip path: %q", raw)
	}

	decoded, err := url.PathUnescape(raw)
	if err != nil {
		return "", false, fmt.Errorf("unsafe plugin zip path: %q", raw)
	}
	if decoded == "" || strings.Contains(decoded, `\`) || strings.HasPrefix(decoded, "/") || hasWindowsDrive(decoded) {
		return "", false, fmt.Errorf("unsafe plugin zip path: %q", raw)
	}

	isDir := strings.HasSuffix(decoded, "/")
	name := strings.TrimSuffix(decoded, "/")
	if name == "" {
		return "", false, fmt.Errorf("unsafe plugin zip path: %q", raw)
	}
	for _, part := range strings.Split(name, "/") {
		if part == "" || part == "." || part == ".." {
			return "", false, fmt.Errorf("unsafe plugin zip path: %q", raw)
		}
	}
	if path.Clean(name) != name {
		return "", false, fmt.Errorf("unsafe plugin zip path: %q", raw)
	}
	return name, isDir, nil
}

func hasWindowsDrive(name string) bool {
	return len(name) >= 2 && name[1] == ':' && ((name[0] >= 'A' && name[0] <= 'Z') || (name[0] >= 'a' && name[0] <= 'z'))
}

func readZipFile(f *zip.File) ([]byte, error) {
	r, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer r.Close()

	return io.ReadAll(r)
}
