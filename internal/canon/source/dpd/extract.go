package dpd

import (
	"archive/tar"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"

	"github.com/ulikunitz/xz"
)

const dbMemberName = "dpd.db"

func Extract(tarballPath, dbPath string, maxDBBytes int64) error {
	if maxDBBytes <= 0 || maxDBBytes == math.MaxInt64 {
		return fmt.Errorf("dpd extract: maxDBBytes must be a positive int64 below MaxInt64, got %d", maxDBBytes)
	}
	if dbPath == "" {
		return fmt.Errorf("dpd extract: empty dbPath")
	}
	f, err := os.Open(tarballPath)
	if err != nil {
		return fmt.Errorf("dpd extract: %w", err)
	}
	defer func() { _ = f.Close() }()
	xr, err := xz.NewReader(f)
	if err != nil {
		return fmt.Errorf("dpd extract: open xz %s: %w", tarballPath, err)
	}
	tr := tar.NewReader(xr)

	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return fmt.Errorf("dpd extract: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(dbPath), ".dpd-db-*")
	if err != nil {
		return fmt.Errorf("dpd extract: %w", err)
	}
	tmpName := tmp.Name()
	defer func() {
		_ = tmp.Close()
		if tmpName != "" {
			_ = os.Remove(tmpName)
		}
	}()

	var names []string
	regular := 0
	wrote := false
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("dpd extract: read tar %s: %w", tarballPath, err)
		}
		names = append(names, hdr.Name)
		switch hdr.Typeflag {
		case tar.TypeDir:
			continue
		case tar.TypeReg:
			regular++
			if wrote || hdr.Name != dbMemberName {
				continue
			}
			if _, _, err := copyLimited(tr, tmp, maxDBBytes); err != nil {
				return fmt.Errorf("dpd extract: member %q: %w", hdr.Name, err)
			}
			wrote = true
		default:
			return fmt.Errorf("dpd extract: member %q is not a regular file (type flag %q)", hdr.Name, string(rune(hdr.Typeflag)))
		}
	}
	if regular == 0 {
		return fmt.Errorf("dpd extract: %s carries no regular members (entries: %v)", tarballPath, names)
	}
	if regular != 1 || !wrote {
		return fmt.Errorf("dpd extract: %s carries %d regular members, want exactly one named %q (entries: %v)",
			tarballPath, regular, dbMemberName, names)
	}

	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("dpd extract: sync %s: %w", tmpName, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("dpd extract: close %s: %w", tmpName, err)
	}
	if err := os.Rename(tmpName, dbPath); err != nil {
		return fmt.Errorf("dpd extract: rename to %s: %w", dbPath, err)
	}
	tmpName = ""
	if err := f.Close(); err != nil {
		return fmt.Errorf("dpd extract: close tarball %s: %w", tarballPath, err)
	}
	if err := os.Remove(tarballPath); err != nil {
		return fmt.Errorf("dpd extract: remove tarball %s: %w", tarballPath, err)
	}
	return nil
}
