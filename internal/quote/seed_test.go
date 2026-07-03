package quote

import (
	"strings"
	"testing"
)

func TestSqlStringEscapesQuotes(t *testing.T) {
	got := sqlString("Buddha's teaching")
	want := "'Buddha''s teaching'"
	if got != want {
		t.Errorf("sqlString = %q, want %q", got, want)
	}
}

func TestRenderSeedSQLShape(t *testing.T) {
	q := newQuote("MN 22", "the Buddha, MN 22", []string{`"Test passage."`}, "sacredness-and-profanity.txt")
	out := RenderSeedSQL([]*Quote{q})
	for _, want := range []string{
		"DROP TABLE IF EXISTS quotes;",
		"CREATE TABLE quotes (",
		"INSERT INTO quotes (id, sutta_id, citation, body_md, body_text, line_count, char_count, sources) VALUES (1, ",
		"'MN 22'",
		"'the Buddha, MN 22'",
		"*\"Test passage.\"* - **the Buddha, MN 22**",
		"'sacredness-and-profanity.txt'",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("seed missing %q in:\n%s", want, out)
		}
	}
}

func TestSqlStringEdges(t *testing.T) {
	cases := map[string]string{
		"":          "''",
		"abc":       "'abc'",
		"a'b":       "'a''b'",
		"a'b'c":     "'a''b''c'",
		"'leading":  "'''leading'",
		"trailing'": "'trailing'''",
	}
	for in, want := range cases {
		if got := sqlString(in); got != want {
			t.Errorf("sqlString(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRenderSeedSQLMultipleQuotes(t *testing.T) {
	q1 := newQuote("MN 1", "the Buddha, MN 1", []string{`"one"`}, "a")
	q2 := newQuote("MN 2", "the Buddha, MN 2", []string{`"two"`}, "b")
	q3 := newQuote("MN 3", "the Buddha, MN 3", []string{"Buddha's word"}, "c")
	out := RenderSeedSQL([]*Quote{q1, q2, q3})
	for _, want := range []string{
		"VALUES (1, 'MN 1',",
		"VALUES (2, 'MN 2',",
		"VALUES (3, 'MN 3',",
		"'Buddha''s word'",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("seed missing %q in:\n%s", want, out)
		}
	}
}

func TestRenderSeedSQLEmpty(t *testing.T) {
	out := RenderSeedSQL(nil)
	if strings.Contains(out, "INSERT INTO quotes") {
		t.Errorf("empty seed should have no INSERTs:\n%s", out)
	}
	for _, want := range []string{"DROP TABLE IF EXISTS quotes;", "CREATE TABLE quotes ("} {
		if !strings.Contains(out, want) {
			t.Errorf("empty seed missing %q in:\n%s", want, out)
		}
	}
}
