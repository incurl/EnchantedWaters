package importer

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// The asset files of the DOS era releases are often distributed as ZIP
// archives (the "MD" disk images). The importer therefore supports
// reading a file out of an archive.
//
// The archive support is implemented with the standard library, which
// keeps the binary dependency free and CGO free.

// Archive is an opened ZIP archive.
type Archive struct {
	// reader is the underlying zip reader.
	reader *zip.ReadCloser
	// path is the path of the archive on the disk.
	path string
}

// OpenArchive opens a ZIP archive.
func OpenArchive(path string) (*Archive, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("open archive %s: %w", path, err)
	}
	return &Archive{reader: r, path: path}, nil
}

// Close closes the archive.
func (a *Archive) Close() error {
	if a == nil || a.reader == nil {
		return nil
	}
	return a.reader.Close()
}

// Names returns the names of the files inside the archive.
func (a *Archive) Names() []string {
	if a == nil || a.reader == nil {
		return nil
	}
	out := make([]string, 0, len(a.reader.File))
	for _, f := range a.reader.File {
		out = append(out, f.Name)
	}
	return out
}

// Find returns the archive entry whose name matches.
//
// The match is case-insensitive and accepts a partial name, so that a
// specification can refer to "MENU.DAT" without knowing the exact path
// inside the archive.
func (a *Archive) Find(name string) (*zip.File, error) {
	if a == nil || a.reader == nil {
		return nil, fmt.Errorf("no archive is open")
	}
	target := strings.ToLower(filepath.Base(name))
	for _, f := range a.reader.File {
		lower := strings.ToLower(f.Name)
		if lower == target || strings.ToLower(filepath.Base(f.Name)) == target {
			return f, nil
		}
	}
	return nil, fmt.Errorf("file %q not found in %s", name, a.path)
}

// ReadSeekCloser is a random access reader that can be closed.
//
// The archive entries are decompressed into a temporary file, so that the
// memory usage stays bounded no matter how large the asset is.
type ReadSeekCloser interface {
	io.ReadSeeker
	io.Closer
}

// Open opens an entry of the archive for reading.
//
// The returned reader is a stream: the entry is decompressed on the fly,
// so a large file is never held in memory as a whole.
func (a *Archive) Open(entry *zip.File) (ReadSeekCloser, int64, error) {
	rc, err := entry.Open()
	if err != nil {
		return nil, 0, err
	}
	// The zip reader is not a seeker, so the entry is copied into a
	// temporary file when a random access is needed.
	//
	// The temporary file keeps the memory usage bounded, which is the
	// requirement of the importer for large binaries.
	tmp, err := os.CreateTemp("", "enchanted-import-*")
	if err != nil {
		rc.Close()
		return nil, 0, err
	}
	if _, err := io.Copy(tmp, rc); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		rc.Close()
		return nil, 0, err
	}
	rc.Close()
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return nil, 0, err
	}
	// The size is taken from the entry metadata.
	size := int64(entry.UncompressedSize64)
	return &removeOnClose{File: tmp, path: tmp.Name()}, size, nil
}

// removeOnClose removes the temporary file of an archive entry when the
// reader is closed.
type removeOnClose struct {
	*os.File
	// path is the path of the temporary file.
	path string
}

// Close closes the file and removes it from the disk.
func (r *removeOnClose) Close() error {
	err := r.File.Close()
	os.Remove(r.path)
	return err
}
