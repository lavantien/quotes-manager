package export

import (
	"context"
	"database/sql"
	"testing"

	"github.com/lavantien/quotes-manager/internal/canon/graph"
	"github.com/lavantien/quotes-manager/internal/canon/model"
	"github.com/lavantien/quotes-manager/internal/config"
)

func buildFeedsGraphDB(t *testing.T, cfg *config.Config) {
	t.Helper()
	dbPath := cfg.Canon.DBFile("feedsrun")
	store, err := graph.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Migrate(); err != nil {
		t.Fatal(err)
	}
	loader := graph.NewLoader(store, 100)
	node := func(typ model.NodeType, key, data string) int64 {
		t.Helper()
		id, err := loader.EnsureNode(typ, key, data)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	seg := func(key, text string) int64 {
		t.Helper()
		return node(model.NodeSegment, key, `{"text":`+quoteJSON(text)+`}`)
	}
	edge := func(src, dst int64, typ model.EdgeType, data string) {
		t.Helper()
		if err := loader.AddEdge(src, dst, typ, nil, data); err != nil {
			t.Fatal(err)
		}
	}

	divMN := node(model.NodeDivision, "mn", `{}`)
	edition := node(model.NodeEdition, "en/sujato/mn", `{}`)
	textMN1 := node(model.NodeText, "mn1", `{"name":"Mūlapariyāsa"}`)
	textMN2 := node(model.NodeText, "mn2", `{"name":"Sabbāsava"}`)
	textMN3 := node(model.NodeText, "mn3", `{}`)

	dhamma := node(model.NodeHeadword, "dpd:dhamma",
		`{"lemma_1":"dhamma","pos":"m","meaning_1":"the teaching","root_key":"dhṛ","family_root":"dhṛ"}`)
	eva := node(model.NodeHeadword, "dpd:eva",
		`{"lemma_1":"eva","pos":"ind","meaning_1":"thus"}`)
	bhikkhu := node(model.NodeHeadword, "dpd:bhikkhu",
		`{"lemma_1":"bhikkhu","pos":"m","meaning_1":"monk","root_key":"bhikkh","family_root":"bhikkh"}`)

	p11 := seg("pli:mn1:1.1", "pali one")
	p12 := seg("pli:mn1:2.1", "pali two")
	p21 := seg("pli:mn2:1.1", "pali three")
	p31 := seg("pli:mn3:1.1", "pali four")
	en11 := seg("en/sujato:mn1:1.1", "en one")

	edge(p11, textMN1, model.EdgeChildOf, "")
	edge(p12, textMN1, model.EdgeChildOf, "")
	edge(p21, textMN2, model.EdgeChildOf, "")
	edge(p31, textMN3, model.EdgeChildOf, "")
	edge(textMN1, divMN, model.EdgeChildOf, "")
	edge(textMN2, divMN, model.EdgeChildOf, "")
	edge(textMN1, edition, model.EdgeCoveredBy, `{"similarity":1}`)
	edge(en11, p11, model.EdgeTranslates, `{"author":"sujato"}`)
	edge(p11, dhamma, model.EdgeHasWord, `{"forms":["dhamma"]}`)
	edge(p12, dhamma, model.EdgeHasWord, `{"forms":["dhamma","dhammo"]}`)
	edge(p11, eva, model.EdgeHasWord, `{"forms":["evam"]}`)
	edge(p21, bhikkhu, model.EdgeHasWord, `{"forms":["bhikkhu"]}`)
	edge(p31, dhamma, model.EdgeHasWord, `{"forms":["dhammo"]}`)
	if err := loader.Flush(); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if err := graph.PublishDB(cfg, "feedsrun", dbPath); err != nil {
		t.Fatal(err)
	}
}

func openSidecar(t *testing.T, cfg *config.Config) *sql.DB {
	t.Helper()
	ddb, err := sql.Open("duckdb", cfg.Canon.DuckDBFile())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ddb.Close() })
	return ddb
}

func TestRebuildWordOccurrences(t *testing.T) {
	cfg := testConfig(t)
	buildFeedsGraphDB(t, cfg)

	counts, err := Rebuild(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	ddb := openSidecar(t, cfg)

	var n int64
	if err := ddb.QueryRow(`SELECT COUNT(*) FROM word_occurrences`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 5 || counts["word_occurrences"] != 5 {
		t.Errorf("word_occurrences rows = %d, counts map = %d; want 5 and 5", n, counts["word_occurrences"])
	}

	var paliID int64
	var muid, segKey, headword, root, family, forms string
	var division string
	err = ddb.QueryRow(`SELECT pali_id, muid, division, seg_key, headword, root, family_root, forms
		FROM word_occurrences WHERE muid = 'mn1' AND seg_key = '2.1'`).
		Scan(&paliID, &muid, &division, &segKey, &headword, &root, &family, &forms)
	if err != nil {
		t.Fatal(err)
	}
	if muid != "mn1" || segKey != "2.1" || division != "mn" || headword != "dhamma" ||
		root != "dhṛ" || family != "dhṛ" || forms != `{"forms":["dhamma","dhammo"]}` {
		t.Errorf("word occurrence = %d/%s/%s/%s/%s/%s/%s/%s",
			paliID, muid, division, segKey, headword, root, family, forms)
	}

	var root2, family2, division3 any
	if err := ddb.QueryRow(`SELECT root, family_root FROM word_occurrences
		WHERE headword = 'eva'`).Scan(&root2, &family2); err != nil {
		t.Fatal(err)
	}
	if root2 != nil || family2 != nil {
		t.Errorf("eva root/family = %v/%v; want NULLs", root2, family2)
	}
	if err := ddb.QueryRow(`SELECT division FROM word_occurrences
		WHERE muid = 'mn3'`).Scan(&division3); err != nil {
		t.Fatal(err)
	}
	if division3 != nil {
		t.Errorf("mn3 division = %v; want NULL", division3)
	}

	var orphan int64
	if err := ddb.QueryRow(`SELECT COUNT(*) FROM word_occurrences wo
		WHERE NOT EXISTS (
			SELECT 1 FROM node owner
			WHERE owner.id = wo.pali_id AND owner.key = 'pli:' || wo.muid || ':' || wo.seg_key
		)`).Scan(&orphan); err != nil {
		t.Fatal(err)
	}
	if orphan != 0 {
		t.Errorf("word_occurrences rows without a matching pali node = %d; want 0", orphan)
	}
}

func TestRebuildTextStats(t *testing.T) {
	cfg := testConfig(t)
	buildFeedsGraphDB(t, cfg)

	counts, err := Rebuild(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	ddb := openSidecar(t, cfg)

	var n int64
	if err := ddb.QueryRow(`SELECT COUNT(*) FROM text_stats`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 3 || counts["text_stats"] != 3 {
		t.Errorf("text_stats rows = %d, counts map = %d; want 3 and 3", n, counts["text_stats"])
	}

	var division, name string
	var segments, translated, hasWord, distinctHW int64
	var covered bool
	err = ddb.QueryRow(`SELECT division, name, segments, translated, has_word_edges, distinct_headwords, covered
		FROM text_stats WHERE muid = 'mn1'`).
		Scan(&division, &name, &segments, &translated, &hasWord, &distinctHW, &covered)
	if err != nil {
		t.Fatal(err)
	}
	if division != "mn" || name != "Mūlapariyāsa" || segments != 2 || translated != 1 ||
		hasWord != 3 || distinctHW != 2 || !covered {
		t.Errorf("mn1 stats = %s/%s/%d/%d/%d/%d/%v", division, name, segments, translated, hasWord, distinctHW, covered)
	}

	err = ddb.QueryRow(`SELECT division, name, segments, translated, has_word_edges, distinct_headwords, covered
		FROM text_stats WHERE muid = 'mn2'`).
		Scan(&division, &name, &segments, &translated, &hasWord, &distinctHW, &covered)
	if err != nil {
		t.Fatal(err)
	}
	if division != "mn" || name != "Sabbāsava" || segments != 1 || translated != 0 ||
		hasWord != 1 || distinctHW != 1 || covered {
		t.Errorf("mn2 stats = %s/%s/%d/%d/%d/%d/%v", division, name, segments, translated, hasWord, distinctHW, covered)
	}

	var division3, name3 any
	err = ddb.QueryRow(`SELECT division, name, segments, translated, has_word_edges, distinct_headwords, covered
		FROM text_stats WHERE muid = 'mn3'`).
		Scan(&division3, &name3, &segments, &translated, &hasWord, &distinctHW, &covered)
	if err != nil {
		t.Fatal(err)
	}
	if division3 != nil || name3 != nil || segments != 1 || translated != 0 ||
		hasWord != 1 || distinctHW != 1 || covered {
		t.Errorf("mn3 stats = %v/%v/%d/%d/%d/%d/%v", division3, name3, segments, translated, hasWord, distinctHW, covered)
	}
}
