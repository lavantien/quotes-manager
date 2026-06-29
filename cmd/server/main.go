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

	st, err := store.Open(*dbPath)
	if err != nil {
		log.Fatalf("open database %s: %v", *dbPath, err)
	}
	defer st.Close()

	if err := seed.EnsureSeeded(st.DB()); err != nil {
		log.Fatalf("seed database: %v", err)
	}

	log.Printf("quotes-manager listening on http://localhost%s", *addr)
	if err := http.ListenAndServe(*addr, server.New(st)); err != nil {
		log.Fatalf("server: %v", err)
	}
}
