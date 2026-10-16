package dpd

import (
	"os"
	"path/filepath"
	"testing"
)

func FuzzDpdExtract(f *testing.F) {
	if seed, err := buildTarXZBytes([]tarMember{{name: dbMemberName, body: "seed db bytes"}}); err == nil {
		f.Add(seed)
	}
	if seed, err := buildTarXZBytes([]tarMember{{name: "a", body: "1"}, {name: "b", body: "2"}}); err == nil {
		f.Add(seed)
	}
	f.Add([]byte("not xz at all"))
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, data []byte) {
		dir := t.TempDir()
		tarball := filepath.Join(dir, "dpd.db.tar.xz")
		if err := os.WriteFile(tarball, data, 0o644); err != nil {
			t.Fatalf("write tarball: %v", err)
		}
		db := filepath.Join(dir, "out", "dpd.db")
		const max = int64(1) << 20
		err := Extract(tarball, db, max)
		if err != nil {
			if _, statErr := os.Stat(db); statErr == nil {
				t.Errorf("failed extract left a db behind")
			}
			return
		}
		fi, statErr := os.Stat(db)
		if statErr != nil {
			t.Fatalf("successful extract left no db: %v", statErr)
		}
		if fi.Size() > max {
			t.Errorf("extracted db size %d exceeds max %d", fi.Size(), max)
		}
		if _, statErr := os.Stat(tarball); !os.IsNotExist(statErr) {
			t.Errorf("successful extract must remove the tarball, stat err = %v", statErr)
		}
	})
}
