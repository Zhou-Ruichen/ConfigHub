package bundle

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// TarGz writes bundleDir as a deterministic tar.gz stream. Entries are sorted
// by slash-separated path and gzip metadata is kept stable for repeatable bytes.
func TarGz(bundleDir string, w io.Writer) error {
	paths := make([]string, 0)
	if err := filepath.WalkDir(bundleDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == bundleDir {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("bundle archive rejects symlink %q", path)
		}
		if d.IsDir() {
			return nil
		}
		paths = append(paths, path)
		return nil
	}); err != nil {
		return fmt.Errorf("walk bundle for archive: %w", err)
	}
	sort.Slice(paths, func(i, j int) bool {
		ri, _ := filepath.Rel(bundleDir, paths[i])
		rj, _ := filepath.Rel(bundleDir, paths[j])
		return filepath.ToSlash(ri) < filepath.ToSlash(rj)
	})

	gz, err := gzip.NewWriterLevel(w, gzip.BestCompression)
	if err != nil {
		return fmt.Errorf("create gzip writer: %w", err)
	}
	gz.Name = ""
	gz.Comment = ""
	tw := tar.NewWriter(gz)
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			_ = tw.Close()
			_ = gz.Close()
			return fmt.Errorf("stat archive file %q: %w", path, err)
		}
		rel, err := filepath.Rel(bundleDir, path)
		if err != nil {
			_ = tw.Close()
			_ = gz.Close()
			return fmt.Errorf("rel archive file %q: %w", path, err)
		}
		name := strings.TrimPrefix(filepath.ToSlash(rel), "./")
		header := &tar.Header{
			Name: name,
			Mode: int64(info.Mode().Perm()),
			Size: info.Size(),
		}
		if err := tw.WriteHeader(header); err != nil {
			_ = tw.Close()
			_ = gz.Close()
			return fmt.Errorf("write tar header %q: %w", name, err)
		}
		file, err := os.Open(path)
		if err != nil {
			_ = tw.Close()
			_ = gz.Close()
			return fmt.Errorf("open archive file %q: %w", path, err)
		}
		_, copyErr := io.Copy(tw, file)
		closeErr := file.Close()
		if copyErr != nil {
			_ = tw.Close()
			_ = gz.Close()
			return fmt.Errorf("write tar file %q: %w", name, copyErr)
		}
		if closeErr != nil {
			_ = tw.Close()
			_ = gz.Close()
			return fmt.Errorf("close archive file %q: %w", path, closeErr)
		}
	}
	if err := tw.Close(); err != nil {
		_ = gz.Close()
		return fmt.Errorf("close tar writer: %w", err)
	}
	if err := gz.Close(); err != nil {
		return fmt.Errorf("close gzip writer: %w", err)
	}
	return nil
}
