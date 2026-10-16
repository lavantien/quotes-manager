package server

import (
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
)

const reportDateLayout = "2006-01-02"

var reportFileRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

var reportReservedNames = map[string]bool{
	"CON": true, "PRN": true, "AUX": true, "NUL": true,
	"COM1": true, "COM2": true, "COM3": true, "COM4": true, "COM5": true,
	"COM6": true, "COM7": true, "COM8": true, "COM9": true,
	"LPT1": true, "LPT2": true, "LPT3": true, "LPT4": true, "LPT5": true,
	"LPT6": true, "LPT7": true, "LPT8": true, "LPT9": true,
}

func reportReserved(file string) bool {
	base, _, _ := strings.Cut(file, ".")
	return reportReservedNames[strings.ToUpper(base)]
}

const codeReportNotFound = "report_not_found"

func (s *Server) registerReportRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/report", s.apiReportList)
	mux.HandleFunc("GET /api/v1/report/{date}/{file}", s.apiReportDownload)
}

type reportEntry struct {
	Date  string   `json:"date"`
	Files []string `json:"files"`
}

type reportListing struct {
	Reports []reportEntry `json:"reports"`
}

func (s *Server) apiReportList(w http.ResponseWriter, _ *http.Request) {
	out := reportListing{Reports: []reportEntry{}}
	entries, err := os.ReadDir(s.cfg.Canon.Paths.ReportsDir)
	if err != nil {
		writeJSON(w, http.StatusOK, out)
		return
	}
	var dates []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, err := time.Parse(reportDateLayout, e.Name()); err != nil {
			continue
		}
		dates = append(dates, e.Name())
	}
	slices.Sort(dates)
	for i := len(dates) - 1; i >= 0; i-- {
		files, err := os.ReadDir(filepath.Join(s.cfg.Canon.Paths.ReportsDir, dates[i]))
		entry := reportEntry{Date: dates[i], Files: []string{}}
		if err == nil {
			for _, fe := range files {
				if fe.Type().IsRegular() {
					entry.Files = append(entry.Files, fe.Name())
				}
			}
		}
		out.Reports = append(out.Reports, entry)
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) apiReportDownload(w http.ResponseWriter, r *http.Request) {
	date, file := r.PathValue("date"), r.PathValue("file")
	if _, err := time.Parse(reportDateLayout, date); err != nil {
		writeErr(w, http.StatusBadRequest, codeInvalidParam,
			"date must be formatted "+reportDateLayout)
		return
	}
	if !reportFileRe.MatchString(file) {
		writeErr(w, http.StatusBadRequest, codeInvalidParam,
			"file must match "+reportFileRe.String())
		return
	}
	if reportReserved(file) {
		writeErr(w, http.StatusBadRequest, codeInvalidParam,
			"file must not be a reserved device name")
		return
	}
	raw, err := os.ReadFile(filepath.Join(s.cfg.Canon.ReportsDirFor(date), file))
	if err != nil {
		writeError(w, errNotFound(codeReportNotFound,
			"no report "+file+" for "+date))
		return
	}
	w.Header().Set("Content-Type", reportContentType(file))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(raw)
}

func reportContentType(file string) string {
	switch filepath.Ext(file) {
	case ".md":
		return "text/markdown; charset=utf-8"
	case ".json":
		return "application/json"
	default:
		return "application/octet-stream"
	}
}
