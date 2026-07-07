package main

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func looksLikeClusterArchive(path string) bool {
	p := strings.ToLower(path)
	return strings.HasSuffix(p, ".tar.gz") || strings.HasSuffix(p, ".tgz")
}

// extractClusterArchive unpacks a .tar.gz into destDir (must exist).
// Returns the logical dump root: if the archive contains a single top-level directory, that path;
// otherwise destDir.
func extractClusterArchive(archivePath, destDir string) (dumpRoot string, err error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return "", err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		if hdr.Name == "" {
			continue
		}
		cleanName := strings.TrimPrefix(filepath.ToSlash(hdr.Name), "/")
		if cleanName == "" || cleanName == "." {
			continue
		}
		target, err := safeExtractPath(destDir, cleanName)
		if err != nil {
			return "", fmt.Errorf("%w: %q", err, hdr.Name)
		}
		fi := hdr.FileInfo()
		switch {
		case fi.IsDir():
			if err := os.MkdirAll(target, 0o755); err != nil {
				return "", err
			}
		case fi.Mode().IsRegular():
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return "", err
			}
			// A tarball can list a directory, then a regular file with the *same* path
			// (some collectors emit duplicate or conflicting path entries). OpenFile
			// on an existing directory fails with EISDIR. If the directory is empty, remove
			// it so the file can be created; if not, the archive is ambiguous.
			if err := clearPathForRegularFile(target); err != nil {
				return "", err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, fi.Mode()&0o777)
			if err != nil {
				return "", err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return "", err
			}
			if err := out.Close(); err != nil {
				return "", err
			}
		default:
			// Skip symlinks, devices, and other special entries.
			continue
		}
	}

	return inferDumpRoot(destDir)
}

// clearPathForRegularFile removes a path that exists only as an *empty* directory, so
// a regular file can be created at the same name. Non-empty directory → error.
func clearPathForRegularFile(target string) error {
	st, err := os.Lstat(target)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if !st.IsDir() {
		return nil
	}
	ents, err := os.ReadDir(target)
	if err != nil {
		return err
	}
	if len(ents) > 0 {
		return fmt.Errorf("cannot create file at %q: a non-empty directory already exists (tar has conflicting path entries: both a directory and a file for this name)", target)
	}
	return os.RemoveAll(target)
}

func safeExtractPath(destDir, name string) (string, error) {
	destDir = filepath.Clean(destDir)
	for _, p := range strings.Split(name, "/") {
		if p == ".." {
			return "", fmt.Errorf("illegal path component")
		}
	}
	target := filepath.Join(destDir, filepath.FromSlash(name))
	rel, err := filepath.Rel(destDir, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("path escapes destination")
	}
	return target, nil
}

func inferDumpRoot(extractDir string) (string, error) {
	entries, err := os.ReadDir(extractDir)
	if err != nil {
		return "", err
	}
	if len(entries) == 1 && entries[0].IsDir() {
		return filepath.Join(extractDir, entries[0].Name()), nil
	}
	return extractDir, nil
}
