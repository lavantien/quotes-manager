package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lavantien/quotes-manager/internal/canon/pipeline"
	"github.com/lavantien/quotes-manager/internal/canon/source/dpd"
	"github.com/lavantien/quotes-manager/internal/config"
)

func dpdTestConfig(t *testing.T) *config.Config {
	t.Helper()
	src := t.TempDir()
	_, tarPath, err := dpd.WriteFixture(src)
	if err != nil {
		t.Fatal(err)
	}
	tmp := t.TempDir()
	cfg := config.Defaults()
	cfg.Canon.Paths.DataDir = tmp
	cfg.Canon.Paths.DBDir = filepath.Join(tmp, "db")
	cfg.Canon.Paths.DistDir = filepath.Join(tmp, "dist")
	cfg.Canon.Paths.ManifestsDir = filepath.Join(tmp, "docs", "manifests")
	cfg.Canon.Paths.ReportsDir = filepath.Join(tmp, "docs", "reports")
	cfg.Canon.Dpd.URL = "file:///" + strings.TrimPrefix(filepath.ToSlash(tarPath), "/")
	return cfg
}

func fileSHA256(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func TestSyncDpdFetchExtractsAndRecordsProvenance(t *testing.T) {
	cfg := dpdTestConfig(t)
	srcTar := filepath.FromSlash(strings.TrimPrefix(cfg.Canon.Dpd.URL, "file:///"))

	if err := runSync(context.Background(), cfg, []string{"--source", "dpd"}); err != nil {
		t.Fatalf("sync dpd: %v", err)
	}
	if _, err := os.Stat(cfg.Canon.DpdDB()); err != nil {
		t.Errorf("extracted dpd.db missing: %v", err)
	}
	if _, err := os.Stat(cfg.Canon.DpdTarball()); !os.IsNotExist(err) {
		t.Errorf("fetched tarball must be consumed by extract, stat err = %v", err)
	}

	raw, err := os.ReadFile(cfg.Canon.DpdProvenanceFile())
	if err != nil {
		t.Fatalf("provenance.json missing: %v", err)
	}
	var pv struct {
		Tag     string `json:"tag"`
		SHA256  string `json:"sha256"`
		Size    int64  `json:"size"`
		Fetched string `json:"fetched"`
	}
	if err := json.Unmarshal(raw, &pv); err != nil {
		t.Fatal(err)
	}
	if pv.Tag != "local" {
		t.Errorf("tag = %q; want local", pv.Tag)
	}
	if want := fileSHA256(t, srcTar); pv.SHA256 != want {
		t.Errorf("sha256 = %s; want %s", pv.SHA256, want)
	}
	if fi, err := os.Stat(srcTar); err != nil || pv.Size != fi.Size() {
		t.Errorf("size = %d; want %d (%v)", pv.Size, fi.Size(), err)
	}
	if _, err := time.Parse(time.RFC3339, pv.Fetched); err != nil {
		t.Errorf("fetched = %q: %v", pv.Fetched, err)
	}

	m, err := pipeline.ReadManifest(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if m.Dpd.Tag != "local" || m.Dpd.SHA256 != pv.SHA256 || m.Dpd.Size != pv.Size || m.Dpd.Fetched != pv.Fetched {
		t.Errorf("manifest dpd block = %+v; want the provenance record %+v", m.Dpd, pv)
	}
}

func TestSyncDpdProvenanceSurvivesManifestLoss(t *testing.T) {
	cfg := dpdTestConfig(t)
	if err := runSync(context.Background(), cfg, []string{"--source", "dpd"}); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(cfg.Canon.ManifestFile()); err != nil {
		t.Fatal(err)
	}

	if err := writeSyncManifest(context.Background(), cfg, nil); err != nil {
		t.Fatal(err)
	}
	m, err := pipeline.ReadManifest(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if m.Dpd.Tag != "local" || m.Dpd.SHA256 == "" {
		t.Errorf("manifest dpd block after rebuild = %+v; want the provenance record", m.Dpd)
	}
}

func TestSyncDpdFetchFailureIsHard(t *testing.T) {
	cfg := dpdTestConfig(t)
	cfg.Canon.Dpd.URL += ".missing"

	if err := runSync(context.Background(), cfg, []string{"--source", "dpd"}); err == nil {
		t.Fatal("dpd sync must fail when the source cannot be fetched")
	}
	if _, err := os.Stat(cfg.Canon.DpdProvenanceFile()); err == nil {
		t.Error("no provenance may be written for a failed fetch")
	}
}

func TestMirrorSourceAllCoversDpd(t *testing.T) {
	kinds, err := mirrorSource("all")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"bilara", "editions", "dpd"} {
		found := false
		for _, k := range kinds {
			if k == want {
				found = true
			}
		}
		if !found {
			t.Errorf("mirrorSource(all) = %v; missing %s", kinds, want)
		}
	}
}
