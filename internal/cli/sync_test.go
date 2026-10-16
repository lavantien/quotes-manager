package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSyncSingleSourceWritesCompleteManifest(t *testing.T) {
	tmp := t.TempDir()
	buildHermeticBilara(t, filepath.Join(tmp, "bilara-src"))
	buildHermeticEditions(t, filepath.Join(tmp, "editions-src"))
	cfgPath := writeHermeticConfig(t, tmp)

	if code, out := runCLI(t, cfgPath, "sync", "--source", "bilara"); code != 0 {
		t.Fatalf("bilara-only sync exit %d: %s", code, out)
	}
	work := filepath.Join(tmp, "work")
	manifestPath := filepath.Join(work, "docs", "manifests", "sync.json")
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("manifest missing after single-source sync: %v", err)
	}
	var m struct {
		Bilara struct {
			Commit string `json:"commit"`
		} `json:"bilara"`
		Editions struct {
			Commit string `json:"commit"`
		} `json:"editions"`
		Htmls []struct {
			FileName string `json:"file_name"`
		} `json:"htmls"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	if m.Bilara.Commit == "" {
		t.Error("bilara commit missing from manifest")
	}
	if m.Editions.Commit != "" || len(m.Htmls) != 0 {
		t.Errorf("editions half must stay empty before its first sync: %+v", m)
	}

	if code, out := runCLI(t, cfgPath, "sync", "--source", "editions"); code != 0 {
		t.Fatalf("editions-only sync exit %d: %s", code, out)
	}
	raw, err = os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	if m.Bilara.Commit == "" {
		t.Error("bilara commit lost after editions-only sync")
	}
	if m.Editions.Commit == "" || len(m.Htmls) != 1 {
		t.Errorf("editions half not recorded: %+v", m)
	}

	for _, tc := range []struct {
		name   string
		source string
		half   string
	}{
		{"editions-only sync repopulates bilara", "editions", "bilara"},
		{"bilara-only sync repopulates editions", "bilara", "editions"},
	} {
		if err := os.Remove(manifestPath); err != nil {
			t.Fatalf("%s: remove manifest: %v", tc.name, err)
		}
		if code, out := runCLI(t, cfgPath, "sync", "--source", tc.source); code != 0 {
			t.Fatalf("%s exit %d: %s", tc.name, code, out)
		}
		raw, err = os.ReadFile(manifestPath)
		if err != nil {
			t.Fatalf("%s: manifest missing: %v", tc.name, err)
		}
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if tc.half == "bilara" {
			if m.Bilara.Commit == "" {
				t.Errorf("%s: bilara half not rebuilt from the mirror", tc.name)
			}
		} else if m.Editions.Commit == "" || len(m.Htmls) != 1 {
			t.Errorf("%s: editions half not rebuilt from the mirror: %+v", tc.name, m)
		}
	}
}
