// Command server runs the quotes-manager web application. It opens (creating if
// needed) the SQLite database, ensures it is seeded and migrated, and serves the
// web UI. Run with CGO_ENABLED=1 (mattn/go-sqlite3).
package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/lavantien/quotes-manager/internal/seed"
	"github.com/lavantien/quotes-manager/internal/server"
	"github.com/lavantien/quotes-manager/internal/store"
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	dbPath := flag.String("db", "database/quotes.db", "SQLite database path")
	flag.Parse()

	if err := serve(*addr, *dbPath); err != nil {
		log.Fatalf("server: %v", err)
	}
}

// serve opens (creating if needed) and seeds the database, then serves the web
// UI on addr until the process is stopped.
func serve(addr, dbPath string) error {
	st, err := store.Open(dbPath)
	if err != nil {
		return err
	}
	defer st.Close()

	if err := seed.EnsureSeeded(st.DB()); err != nil {
		return err
	}

	log.Printf("quotes-manager listening on http://localhost%s", addr)
	return http.ListenAndServe(addr, server.New(st))
}
