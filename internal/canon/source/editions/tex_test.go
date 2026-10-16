package editions

import (
	"strings"
	"testing"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

func TestNormalize(t *testing.T) {
	bom := string(rune(0xFEFF))
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"comment inline", "foo % a comment\nbar", "foo \nbar"},
		{"comment whole line", "% only comment\nkeep", "keep"},
		{"escaped percent kept", `costs 50\% more`, `costs 50\% more`},
		{"footnote nested drop", `a\footnote{see \emph{this}}b`, "ab"},
		{"footnote optional arg", `a\footnote[12]{note}b`, "ab"},
		{"styling macros", `\emph{a} \textit{b} \textbf{c} \textsc{d}`, "a b c d"},
		{"quotation macros", `\enquote{q} \parencite{key2020} \textcite{key}`, "q key2020 key"},
		{"unknown macro keeps arg", `pre \unknown{arg} tail`, "pre arg tail"},
		{"bare macro dropped", `a \unknownmacro b`, "a b"},
		{"line break macro", `a \\ b`, "a \nb"},
		{"line break with skip", `x \\[6pt] y`, "x \ny"},
		{"par newline", `a \par b`, "a \nb"},
		{"newline macro", `a \newline b`, "a \nb"},
		{"escapes unwrap", `\& \# \$ \_ \{ \}`, "& # $ _ { }"},
		{"tilde space", "the~Buddha", "the Buddha"},
		{"escaped tilde brace", `a\~{}b`, "a b"},
		{"spacing macros", `a\,b\;c\:d\!e`, "a b c d e"},
		{"double quotes", "``double'' quoted", `"double" quoted`},
		{"mixed quote splices", "`'", "\""},
		{"mixed quote splices longer", "`''", "'\""},
		{"apostrophe", "don't and 'quoted'", "don't and 'quoted'"},
		{"em dash", "well---known", "well-known"},
		{"en dash", "re--born", "re-born"},
		{"dash run", "x----y", "x-y"},
		{"dash across macro boundary", `\emph{a-}-b`, "a-b"},
		{"nfc compose", "cafe\u0301", "café"},
		{"crlf and cr", "a\r\nb\rc", "a\nb\nc"},
		{"space collapse", "a \t\t b  c", "a b c"},
		{"newlines preserved", "a \n\n b", "a \n\n b"},
		{"trim ends", "  hi  ", "hi"},
		{"bom dropped", string(rune(0xFEFF)) + "hi", "hi"},
		{"nested macro args", `\emph{nested \textbf{bold} text}`, "nested bold text"},
		{"comment inside arg", "\\textit{a % gone\nb}", "a \nb"},
		{"raw group braces kept", `\emph{a{b}c}`, "a{b}c"},
		{"control symbol debackslashed", `a\-b \'c`, "a-b 'c"},
		{"empty", "", ""},
		{"bom after dropped macro", "\\" + bom + "x", "x"},
		{"stacked boms", bom + bom + "x", "x"},
		{"mid text bom stripped", "a" + bom + "b", "ab"},
		{"bom after comment", "% c\n" + bom + "x", "x"},
		{"unclosed optional arg keeps rest", "keep1\\\\[oops\nkeep2", "keep1\n[oops\nkeep2"},
		{"optional arg after space", "a\\\\ [6pt] b", "a\nb"},
		{"macro arg on next line", "a\\emph\n{bold}b", "aboldb"},
		{"footnote arg on next line", "a\\footnote\n{note}b", "ab"},
		{"footnote brace in comment", "a\\footnote{note%}\nmore}b", "ab"},
		{"footnote escaped close brace", `a\footnote{x\}y}b`, "ab"},
		{"footnote double backslash before close", `a\footnote{x\\}y}b`, "ay}b"},
		{"trailing lone backslash", `a\`, "a"},
		{"star break", `a \\* b`, "a \nb"},
		{"tilde macro no braces", `a\~b`, "a b"},
		{"tilde macro truncated brace", `a\~{`, "a {"},
		{"footnote without group", `a\footnote x`, "ax"},
		{"footnote at end", `a\footnote`, "a"},
		{"bare macro at end", `a \emph`, "a"},
		{"comment to eof inside arg probe", `a\emph% c`, "a"},
		{"environment names dropped", `\begin{verse}lined out\end{verse} tail`, "lined out tail"},
		{"environment with space", "a\n\\begin {enumerate}\nitem text\n\\end{enumerate}\nb", "a\n\nitem text\n\nb"},
		{"bare begin without name", `a\begin b`, "ab"},
		{"starred section heading", `\section*{Origin story } body`, "Origin story body"},
		{"starred chapter heading", `\chapter*{Colophon}`, "Colophon"},
		{"starred macro with space", `\section* {Title}`, "Title"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Normalize(tc.in)
			if got != tc.want {
				t.Errorf("Normalize(%q) =\n%q\nwant\n%q", tc.in, got, tc.want)
			}
			again := Normalize(got)
			if again != got {
				t.Errorf("not idempotent for %q: %q then %q", tc.in, got, again)
			}
			if strings.ContainsRune(got, 0xFEFF) {
				t.Errorf("output still carries a byte order mark: %q", got)
			}
		})
	}
}

func TestNormalizeBomIdempotence(t *testing.T) {
	bom := string(rune(0xFEFF))
	inputs := []string{
		"\\" + bom + "x",
		bom + bom + "x",
		"% c\n" + bom + "x",
		"\\foo" + bom + "x",
		"a" + bom,
		bom,
	}
	for _, in := range inputs {
		once := Normalize(in)
		twice := Normalize(once)
		if once != twice {
			t.Errorf("Normalize(%q) = %q, renormalizes to %q", in, once, twice)
		}
		if strings.ContainsRune(once, 0xFEFF) {
			t.Errorf("Normalize(%q) keeps a byte order mark: %q", in, once)
		}
	}
}

func TestNormalizeKitchenSink(t *testing.T) {
	in := "\\documentclass{book}\n" +
		"% preamble comment\n" +
		"\\chapter{The Buddha}\n" +
		"He said---``satipa\\tth\\={a}na~is~the~way''---to \\emph{us}.\\footnote{MN 10.}\n" +
		"10\\% of 50\\$ stays.\\par\n" +
		"``direct'' and 'single'\n"
	got := Normalize(in)
	want := "book\n" +
		"\n" +
		"The Buddha\n" +
		"He said-\"satipa={a}na is the way\"-to us.\n" +
		"10\\% of 50$ stays.\n" +
		"\n" +
		"\"direct\" and 'single'"
	if got != want {
		t.Errorf("Normalize kitchen sink =\n%q\nwant\n%q", got, want)
	}
	if Normalize(got) != got {
		t.Error("kitchen sink not idempotent")
	}
	if !utf8.ValidString(got) || !norm.NFC.IsNormalString(got) {
		t.Error("kitchen sink output not valid NFC UTF-8")
	}
}

func TestNormalizeLongInput(t *testing.T) {
	var b strings.Builder
	for range 10_000 {
		b.WriteString(`\emph{row} \\ `)
	}
	got := Normalize(b.String())
	if n := strings.Count(got, "row"); n != 10_000 {
		t.Errorf("row count = %d, want 10000", n)
	}
	if n := strings.Count(got, "\n"); n != 9_999 {
		t.Errorf("newline count = %d, want 9999", n)
	}
}
