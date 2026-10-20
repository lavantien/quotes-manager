package e2e

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func fetchHeader(app *mergedApp, t *testing.T, method, path, key, value string) fetched {
	t.Helper()
	req, err := http.NewRequest(method, app.server().URL+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if key != "" {
		req.Header.Set(key, value)
	}
	resp, err := app.server().Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return fetched{status: resp.StatusCode, header: resp.Header, body: string(raw)}
}

func TestCollectionGroupsRenderedFromSeed(t *testing.T) {
	app := mergedShared(t)
	col := app.get(t, "/collections/1")
	if col.status != http.StatusOK {
		t.Fatalf("/collections/1: status = %d (%s)", col.status, clip(col.body))
	}
	body := col.body
	i1 := strings.Index(body, `id="group-1"`)
	i16 := strings.Index(body, `id="group-16"`)
	if i1 < 0 || i16 < 0 || i1 > i16 {
		t.Fatalf("seeded group sections missing or out of order: group-1 %d, group-16 %d", i1, i16)
	}
	for _, name := range []string{
		"Five Daily Reflections and Six Contemplations",
		"Dependent Origination",
	} {
		if !strings.Contains(body, name) {
			t.Errorf("seeded group name %q missing", name)
		}
	}
	if n := strings.Count(body, `class="collection-group"`); n != 17 {
		t.Errorf("group sections = %d, want 16 seeded plus the ungrouped section", n)
	}
	if !strings.Contains(body, "471 items") {
		t.Error("collection count must show the 471 seeded suttas")
	}
	if !strings.Contains(body, "https://suttacentral.net/mn26/") {
		t.Error("grouped view must link the seeded sutta rows")
	}
	ids := app.get(t, "/collections/1/ids.txt")
	if ids.status != http.StatusOK || !strings.Contains(ids.body, "mn26") {
		t.Errorf("ids.txt = %d (%s), want the collection's sutta muids", ids.status, clip(ids.body))
	}
	if strings.Contains(ids.body, "\n\n") || !strings.HasSuffix(ids.body, "\n") {
		t.Error("ids.txt must stay one clean id per line")
	}
}

func idListHas(body, id string) bool {
	for _, line := range strings.Split(body, "\n") {
		if line == id {
			return true
		}
	}
	return false
}

func TestCollectionGroupsUserFlow(t *testing.T) {
	app := mergedAppFor(t, true)
	origin := app.server().URL

	created := app.postForm(t, "/collections/1/groups", url.Values{"name": {"e2e group"}}, origin)
	if created.status != http.StatusOK || !strings.Contains(created.body, "e2e group") {
		t.Fatalf("create group = %d (%s)", created.status, clip(created.body))
	}

	renamed := fetchHeader(app, t, http.MethodPost, "/collections/1/groups/17/rename", "HX-Prompt", "renamed%20group")
	if renamed.status != http.StatusOK || !strings.Contains(renamed.body, "renamed group") {
		t.Fatalf("rename group = %d (%s)", renamed.status, clip(renamed.body))
	}

	added := app.postForm(t, "/collections/1/suttas", url.Values{"muid": {"MN 7"}}, origin)
	if added.status != http.StatusOK || !strings.Contains(added.body, "MN 7") {
		t.Fatalf("add sutta = %d (%s)", added.status, clip(added.body))
	}

	assigned := app.postForm(t, "/collections/1/suttas/mn7/group", url.Values{"group": {"1"}}, origin)
	if assigned.status != http.StatusOK {
		t.Fatalf("assign sutta group = %d (%s)", assigned.status, clip(assigned.body))
	}
	zone := app.get(t, "/collections/1")
	s1 := zone.body[strings.Index(zone.body, `id="group-1"`):strings.Index(zone.body, `id="group-2"`)]
	if !strings.Contains(s1, "MN 7") {
		t.Error("assigned sutta must render inside group 1")
	}
	ids := app.get(t, "/collections/1/ids.txt")
	if !idListHas(ids.body, "mn7") {
		t.Error("ids.txt must gain the added sutta muid")
	}

	moved := app.postForm(t, "/collections/1/groups/1/move?dir=down", nil, origin)
	if moved.status != http.StatusOK {
		t.Fatalf("move group = %d (%s)", moved.status, clip(moved.body))
	}
	after := app.get(t, "/collections/1").body
	if strings.Index(after, `id="group-2"`) > strings.Index(after, `id="group-1"`) {
		t.Error("group 2 must render before group 1 after the move")
	}

	removed := fetchHeader(app, t, http.MethodDelete, "/collections/1/suttas/mn7", "", "")
	if removed.status != http.StatusOK {
		t.Fatalf("remove sutta = %d (%s)", removed.status, clip(removed.body))
	}
	if idListHas(app.get(t, "/collections/1/ids.txt").body, "mn7") {
		t.Error("ids.txt must drop the removed sutta muid")
	}

	deleted := fetchHeader(app, t, http.MethodDelete, "/collections/1/groups/17", "", "")
	if deleted.status != http.StatusOK {
		t.Fatalf("delete group = %d (%s)", deleted.status, clip(deleted.body))
	}
	if strings.Contains(app.get(t, "/collections/1").body, "renamed group") {
		t.Error("deleted group must disappear from the zone")
	}
}
