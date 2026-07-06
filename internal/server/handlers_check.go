package server

import (
	"net/http"
	"strings"

	"github.com/lavantien/quotes-manager/internal/quote"
	"github.com/lavantien/quotes-manager/internal/store"
)

// checkResult is one input id's membership outcome.
type checkResult struct {
	Input string // the raw line as the user typed it
	ID    string // the canonicalized id matched against the corpus
	Found bool
	Count int // how many corpus quotes carry this id (0 when !Found)
}

// checkPaneData drives the check-id workspace: a textarea form in the check
// zone, plus the per-id results list after a submission.
type checkPaneData struct {
	ActiveCatID int64
	ActiveColID int64
	Corpus      int // total corpus size, shown in the header
	Submitted   bool
	Found       int
	Total       int
	Results     []checkResult
}

// splitIDLines splits a pasted blob into trimmed, non-empty id lines. It
// handles "\n" and "\r\n" endings and drops blank/whitespace-only lines so a
// stray empty line never reads as a "not found" id.
func splitIDLines(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		if line = strings.TrimSpace(strings.TrimRight(line, "\r")); line != "" {
			out = append(out, line)
		}
	}
	return out
}

// runCheck reports each input id's presence in the corpus. The corpus is keyed
// by the lowercased sutta_id; each input is canonicalized via
// quote.CanonicalSuttaID (which extracts "MN 22" from "the Buddha, MN 22") and
// falls back to the literal trimmed input when no canonical form is found, so
// matching is case-insensitive. Input order and duplicates are preserved: one
// row per input line.
func runCheck(corpus []store.Quote, inputs []string) []checkResult {
	counts := make(map[string]int, len(corpus))
	for _, q := range corpus {
		if id := strings.ToLower(strings.TrimSpace(q.SuttaID)); id != "" {
			counts[id]++
		}
	}
	var results []checkResult
	for _, in := range inputs {
		canon := quote.CanonicalSuttaID(in)
		if canon == "" {
			canon = in
		}
		n := counts[strings.ToLower(canon)]
		results = append(results, checkResult{Input: in, ID: canon, Found: n > 0, Count: n})
	}
	return results
}

// checkPaneHandler swaps the right column (#collection-zone) into the check-id
// workspace and refreshes the right rail out-of-band so the Check button shows
// active. It reuses railData (the check pane renders no quote list) and reads
// the corpus size from the rail's TotalQuotes, so no extra List() call is made.
func (s *Server) checkPaneHandler(w http.ResponseWriter, r *http.Request) {
	cat, col := parseQueryID(r, "cat"), parseQueryID(r, "col")
	data, err := s.railData(cat, col)
	if err != nil {
		serverError(w, err)
		return
	}
	data.View = "check"
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	s.exec(w, "check_zone", checkPaneData{ActiveCatID: cat, ActiveColID: col, Corpus: data.TotalQuotes})
	s.exec(w, "rail_right_oob", data)
}

// checkIdsHandler runs the membership check for a posted list of ids (form
// field "ids") and returns just the #check-results fragment. An empty/blank
// submission is treated as "not submitted" so the hint stays up.
func (s *Server) checkIdsHandler(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		badRequest(w)
		return
	}
	corpus, err := s.store.List()
	if err != nil {
		serverError(w, err)
		return
	}
	inputs := splitIDLines(r.PostForm.Get("ids"))
	results := runCheck(corpus, inputs)
	found := 0
	for _, res := range results {
		if res.Found {
			found++
		}
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	s.exec(w, "check_results", checkPaneData{Submitted: len(inputs) > 0, Found: found, Total: len(results), Results: results})
}
