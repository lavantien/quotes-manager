package server

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/lavantien/quotes-manager/internal/quote"
)

func (s *Server) createGroup(w http.ResponseWriter, r *http.Request) {
	cid, ok := parseID(w, r, "cid")
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		badRequest(w)
		return
	}
	name := strings.TrimSpace(r.PostForm.Get("name"))
	if name == "" {
		badRequest(w)
		return
	}
	if _, err := s.store.CreateCollectionGroup(cid, name); err != nil {
		handleStoreErr(w, err)
		return
	}
	s.swapCollectionZone(w, parseQueryID(r, "cat"), cid, "", "", false, false)
}

func (s *Server) renameGroup(w http.ResponseWriter, r *http.Request) {
	cid, ok := parseID(w, r, "cid")
	if !ok {
		return
	}
	gid, ok := parseID(w, r, "gid")
	if !ok {
		return
	}
	name := ""
	if err := r.ParseForm(); err == nil {
		name = strings.TrimSpace(r.PostForm.Get("name"))
	}
	if name == "" {
		name = promptValue(r)
	}
	if name == "" {
		badRequest(w)
		return
	}
	if err := s.store.RenameCollectionGroup(cid, gid, name); err != nil {
		handleStoreErr(w, err)
		return
	}
	s.swapCollectionZone(w, parseQueryID(r, "cat"), cid, "", "", false, false)
}

func promptValue(r *http.Request) string {
	raw := r.Header.Get("HX-Prompt")
	if raw == "" {
		return ""
	}
	if decoded, err := url.QueryUnescape(raw); err == nil {
		return strings.TrimSpace(decoded)
	}
	return strings.TrimSpace(raw)
}

func (s *Server) deleteGroup(w http.ResponseWriter, r *http.Request) {
	cid, ok := parseID(w, r, "cid")
	if !ok {
		return
	}
	gid, ok := parseID(w, r, "gid")
	if !ok {
		return
	}
	if err := s.store.DeleteCollectionGroup(cid, gid); err != nil {
		handleStoreErr(w, err)
		return
	}
	s.swapCollectionZone(w, parseQueryID(r, "cat"), cid, "", "", false, false)
}

func (s *Server) moveGroup(w http.ResponseWriter, r *http.Request) {
	cid, ok := parseID(w, r, "cid")
	if !ok {
		return
	}
	gid, ok := parseID(w, r, "gid")
	if !ok {
		return
	}
	dir := strings.ToLower(parseQueryStr(r, "dir"))
	if dir == "" {
		if err := r.ParseForm(); err == nil {
			dir = strings.ToLower(strings.TrimSpace(r.PostForm.Get("dir")))
		}
	}
	if dir != "up" && dir != "down" {
		badRequest(w)
		return
	}
	groups, err := s.store.ListCollectionGroups(cid)
	if err != nil {
		serverError(w, err)
		return
	}
	idx := -1
	ids := make([]int64, len(groups))
	for i, g := range groups {
		ids[i] = g.ID
		if g.ID == gid {
			idx = i
		}
	}
	if idx < 0 {
		notFound(w)
		return
	}
	swap := idx - 1
	if dir == "down" {
		swap = idx + 1
	}
	if swap >= 0 && swap < len(ids) {
		ids[idx], ids[swap] = ids[swap], ids[idx]
	}
	if err := s.store.ReorderCollectionGroups(cid, ids); err != nil {
		handleStoreErr(w, err)
		return
	}
	s.swapCollectionZone(w, parseQueryID(r, "cat"), cid, "", "", false, false)
}

func (s *Server) setQuoteGroup(w http.ResponseWriter, r *http.Request) {
	cid, ok := parseID(w, r, "cid")
	if !ok {
		return
	}
	qid, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	gid, ok := groupIDFromForm(w, r)
	if !ok {
		return
	}
	if err := s.store.SetQuoteGroup(cid, qid, gid); err != nil {
		handleStoreErr(w, err)
		return
	}
	s.swapCollectionZone(w, parseQueryID(r, "cat"), cid, "", "", false, false)
}

func (s *Server) addSutta(w http.ResponseWriter, r *http.Request) {
	cid, ok := parseID(w, r, "cid")
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		badRequest(w)
		return
	}
	muid := strings.TrimSpace(r.PostForm.Get("muid"))
	if muid == "" {
		badRequest(w)
		return
	}
	if normalized, err := quote.MuidFromSuttaID(muid); err == nil {
		muid = normalized
	}
	display, err := quote.DisplaySuttaID(muid)
	if err != nil {
		badRequest(w)
		return
	}
	if err := s.store.AddSuttaToCollection(cid, muid, display, suttaLink(muid)); err != nil {
		handleStoreErr(w, err)
		return
	}
	s.swapCollectionZone(w, parseQueryID(r, "cat"), cid, "", "", true, false)
}

func (s *Server) removeSutta(w http.ResponseWriter, r *http.Request) {
	cid, ok := parseID(w, r, "cid")
	if !ok {
		return
	}
	muid := strings.TrimSpace(r.PathValue("muid"))
	if muid == "" {
		badRequest(w)
		return
	}
	if err := s.store.RemoveSuttaFromCollection(cid, muid); err != nil {
		handleStoreErr(w, err)
		return
	}
	s.swapCollectionZone(w, parseQueryID(r, "cat"), cid, "", "", true, false)
}

func (s *Server) setSuttaGroup(w http.ResponseWriter, r *http.Request) {
	cid, ok := parseID(w, r, "cid")
	if !ok {
		return
	}
	muid := strings.TrimSpace(r.PathValue("muid"))
	if muid == "" {
		badRequest(w)
		return
	}
	gid, ok := groupIDFromForm(w, r)
	if !ok {
		return
	}
	if err := s.store.SetSuttaGroup(cid, muid, gid); err != nil {
		handleStoreErr(w, err)
		return
	}
	s.swapCollectionZone(w, parseQueryID(r, "cat"), cid, "", "", false, false)
}

func groupIDFromForm(w http.ResponseWriter, r *http.Request) (int64, bool) {
	if err := r.ParseForm(); err != nil {
		badRequest(w)
		return 0, false
	}
	raw := strings.TrimSpace(r.PostForm.Get("group"))
	if raw == "" {
		badRequest(w)
		return 0, false
	}
	gid, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || gid < 0 {
		badRequest(w)
		return 0, false
	}
	return gid, true
}

func suttaLink(muid string) string {
	return "https://suttacentral.net/" + muid + "/en/sujato?lang=en&layout=plain&reference=none&notes=none&highlight=true&script=latin"
}
