package server_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestCheckPaneRendersZone(t *testing.T) {
	fs, cid := fakeWithCollection(t)
	srv := newServer(t, fs)
	rec := do(t, srv, "GET", fmt.Sprintf("/pane/check?col=%d", cid), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	// The check zone replaces #collection-zone and — critically — must NOT carry
	// data-cid, or currentCol() in app.js would treat the hidden collection as
	// active and silently mutate it.
	if !strings.Contains(body, `id="collection-zone">`) {
		t.Errorf("check zone should render <section ... id=\"collection-zone\"> with no data-cid; body:\n%s", body)
	}
	if !strings.Contains(body, "zone--check") {
		t.Error("check zone should carry the zone--check class")
	}
	for _, want := range []string{`id="check-input"`, `name="ids"`, `hx-post="/check"`} {
		if !strings.Contains(body, want) {
			t.Errorf("check zone missing %s", want)
		}
	}
	// Out-of-band right rail refreshes so the Check button shows active.
	if !strings.Contains(body, `id="right-rail"`) {
		t.Error("check pane should refresh the right rail out-of-band")
	}
}

func TestCheckPaneToggleBack(t *testing.T) {
	fs, _ := fakeWithCollection(t)
	srv := newServer(t, fs)
	body := do(t, srv, "GET", "/pane/check?col=1", "").Body.String()
	// In check mode the Check button is marked active and toggles BACK to the
	// collection pane, so it is the "leave check mode" affordance.
	if !strings.Contains(body, "btn--primary") {
		t.Error("check-mode Check button should be marked active (btn--primary)")
	}
	if !strings.Contains(body, `hx-get="/pane/collection`) {
		t.Error("check-mode Check button should link back to /pane/collection")
	}
}

func TestCheckPaneRailError(t *testing.T) {
	// railData loads collections; a failing ListCollections must surface as 500.
	assert500(t, newServer(t, failListCols{newFake(sampleQuote(1))}), "GET", "/pane/check", "")
}

func TestCheckResultsFoundAndNotFound(t *testing.T) {
	srv := newServer(t, newFake(
		mkQuote(1, "MN 22", "one"),
		mkQuote(2, "MN 22", "two"),
		mkQuote(3, "AN 5", "three"),
	))
	body := do(t, srv, "POST", "/check", "ids=MN+22%0aNOPE+1",
		"Content-Type", "application/x-www-form-urlencoded").Body.String()
	if !strings.Contains(body, "is-found") || !strings.Contains(body, "is-missing") {
		t.Errorf("expected one found and one missing row; body:\n%s", body)
	}
	if !strings.Contains(body, "✓ 2") {
		t.Errorf("MN 22 has 2 corpus quotes; expected '✓ 2' badge; body:\n%s", body)
	}
	if !strings.Contains(body, "NOPE 1") {
		t.Errorf("the missing input id should be echoed verbatim; body:\n%s", body)
	}
	if !strings.Contains(body, "1 of 2 found") {
		t.Errorf("summary should read '1 of 2 found'; body:\n%s", body)
	}
}

func TestCheckResultsCanonicalizes(t *testing.T) {
	srv := newServer(t, newFake(mkQuote(1, "MN 22", "one")))
	body := do(t, srv, "POST", "/check", "ids=the+Buddha%2C+MN+22",
		"Content-Type", "application/x-www-form-urlencoded").Body.String()
	if !strings.Contains(body, "is-found") {
		t.Errorf("'the Buddha, MN 22' canonicalizes to MN 22 and should be found; body:\n%s", body)
	}
}

func TestCheckResultsCaseInsensitive(t *testing.T) {
	srv := newServer(t, newFake(mkQuote(1, "MN 22", "one")))
	body := do(t, srv, "POST", "/check", "ids=mn+22",
		"Content-Type", "application/x-www-form-urlencoded").Body.String()
	if !strings.Contains(body, "is-found") {
		t.Errorf("lowercase 'mn 22' should match the canonical 'MN 22'; body:\n%s", body)
	}
}

func TestCheckResultsDuplicateInputs(t *testing.T) {
	srv := newServer(t, newFake(mkQuote(1, "AN 5", "one")))
	body := do(t, srv, "POST", "/check", "ids=AN+5%0aAN+5",
		"Content-Type", "application/x-www-form-urlencoded").Body.String()
	if got := strings.Count(body, "is-found"); got != 2 {
		t.Errorf("duplicate inputs should each produce a row; got %d found rows; body:\n%s", got, body)
	}
}

func TestCheckResultsEmptyInput(t *testing.T) {
	srv := newServer(t, newFake(mkQuote(1, "MN 22", "one")))
	body := do(t, srv, "POST", "/check", "ids=",
		"Content-Type", "application/x-www-form-urlencoded").Body.String()
	if !strings.Contains(body, "Paste one text id per line") {
		t.Errorf("empty input should render the hint, not a summary; body:\n%s", body)
	}
	if strings.Contains(body, "is-found") || strings.Contains(body, "is-missing") {
		t.Errorf("empty input should render no result rows; body:\n%s", body)
	}
}

func TestCheckResultsNonCanonicalInput(t *testing.T) {
	// "MN22" (no space) is not a canonical sutta id and is matched literally,
	// so it does NOT match the corpus's "MN 22". Behavior is locked here.
	srv := newServer(t, newFake(mkQuote(1, "MN 22", "one")))
	body := do(t, srv, "POST", "/check", "ids=MN22",
		"Content-Type", "application/x-www-form-urlencoded").Body.String()
	if !strings.Contains(body, "is-missing") {
		t.Errorf("'MN22' (no space) should be missing; body:\n%s", body)
	}
}

func TestCheckResultsStoreError(t *testing.T) {
	assert500(t, newServer(t, failList{newFake(sampleQuote(1))}), "POST", "/check", "ids=MN+1",
		"Content-Type", "application/x-www-form-urlencoded")
}

func TestRightRailCheckButton(t *testing.T) {
	// Default (no check mode): the Check button enters check mode (points at
	// /pane/check) and is not marked active.
	body := do(t, newServer(t, newFake(sampleQuote(1))), "GET", "/rail/right", "").Body.String()
	if !strings.Contains(body, ">Check ids<") {
		t.Error("right rail missing a Check ids button")
	}
	if !strings.Contains(body, `hx-get="/pane/check`) {
		t.Error("Check button should enter check mode via /pane/check")
	}
	if strings.Contains(body, "btn--primary>Check ids<") {
		t.Error("Check button should not be active when not in check mode")
	}
}
