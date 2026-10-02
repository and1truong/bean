package playground

import (
	"io"
	"io/fs"
	"path"
	"sort"
	"strings"
	"time"
)

const (
	// MaxFiles bounds the virtual file map a compile request may carry.
	MaxFiles = 64
	// MaxFileBytes bounds each source document.
	MaxFileBytes = 256 << 10
	// MaxTotalBytes bounds the combined source size.
	MaxTotalBytes = 1 << 20
	// MaxPathBytes bounds a single file path length.
	MaxPathBytes = 256
	// MaxPathDepth bounds nesting inside the virtual file map.
	MaxPathDepth = 8
)

// fileError codes are stable identifiers the browser UI keys on.
const (
	codeFileLimit = "BEAN-P4101"
	codeFilePath  = "BEAN-P4102"
	codeManifest  = "BEAN-P4103"
)

// fileMap is a read-only fs.FS over an explicit name → contents map.
type fileMap map[string]string

func validateFiles(files map[string]string, manifestPath string) *Failure {
	if len(files) == 0 {
		return &Failure{Code: codeFileLimit, Message: "no source files provided"}
	}
	if len(files) > MaxFiles {
		return &Failure{Code: codeFileLimit, Message: "too many source files (maximum 64)"}
	}
	total := 0
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if invalid := invalidPath(name); invalid != "" {
			return &Failure{Code: codeFilePath, Message: name + ": " + invalid}
		}
		size := len(files[name])
		if size > MaxFileBytes {
			return &Failure{Code: codeFileLimit, Message: name + " exceeds the 256 KiB file limit"}
		}
		total += size
	}
	if total > MaxTotalBytes {
		return &Failure{Code: codeFileLimit, Message: "combined source size exceeds the 1 MiB limit"}
	}
	if invalid := invalidManifestPath(manifestPath); invalid != "" {
		return &Failure{Code: codeManifest, Message: manifestPath + ": " + invalid}
	}
	if _, exists := files[manifestPath]; !exists {
		return &Failure{Code: codeManifest, Message: "manifest " + manifestPath + " is not in the provided files"}
	}
	return nil
}

// invalidPath reports why a virtual file map key is rejected, or "" when valid.
func invalidPath(name string) string {
	if !fs.ValidPath(name) || name == "." {
		return "must be a relative file path"
	}
	if len(name) > MaxPathBytes {
		return "path exceeds the 256 byte limit"
	}
	if strings.Contains(name, "..") || strings.HasPrefix(name, "/") || strings.Contains(name, "\\") {
		return "must not contain '..' segments, absolute, or Windows path separators"
	}
	if strings.Count(name, "/") >= MaxPathDepth {
		return "path exceeds the maximum directory depth"
	}
	switch strings.ToLower(path.Ext(name)) {
	case ".yaml", ".yml", ".json":
		return ""
	default:
		return "must be a .yaml, .yml, or .json source file"
	}
}

// invalidManifestPath is invalidPath without the extension requirement message
// for directories — the manifest itself must still be a source file.
func invalidManifestPath(name string) string { return invalidPath(name) }

func (m fileMap) Open(name string) (fs.File, error) {
	contents, exists := m[name]
	if !exists {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	return &virtualFile{name: name, reader: strings.NewReader(contents), size: int64(len(contents))}, nil
}

func (m fileMap) ReadFile(name string) ([]byte, error) {
	contents, exists := m[name]
	if !exists {
		return nil, &fs.PathError{Op: "read", Path: name, Err: fs.ErrNotExist}
	}
	return []byte(contents), nil
}

type virtualFile struct {
	name   string
	reader *strings.Reader
	size   int64
}

func (f *virtualFile) Stat() (fs.FileInfo, error) { return virtualFileInfo{f}, nil }
func (f *virtualFile) Read(p []byte) (int, error) { return f.reader.Read(p) }
func (f *virtualFile) Close() error               { return nil }
func (f *virtualFile) ReadAt(p []byte, off int64) (int, error) {
	return f.reader.ReadAt(p, off)
}
func (f *virtualFile) Seek(offset int64, whence int) (int64, error) {
	return f.reader.Seek(offset, whence)
}

var _ io.Seeker = (*virtualFile)(nil)

type virtualFileInfo struct{ f *virtualFile }

func (i virtualFileInfo) Name() string       { return i.f.name }
func (i virtualFileInfo) Size() int64        { return i.f.size }
func (i virtualFileInfo) Mode() fs.FileMode  { return 0o444 }
func (i virtualFileInfo) ModTime() time.Time { return time.Unix(0, 0) }
func (i virtualFileInfo) IsDir() bool        { return false }
func (i virtualFileInfo) Sys() any           { return nil }
