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
