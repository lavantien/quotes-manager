package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/lavantien/quotes-manager/internal/canon/source/dpd"
)

func main() {
	dir := flag.String("dir", filepath.Join("testdata", "dpd"), "output directory")
	flag.Parse()
	dbPath, tarPath, err := dpd.WriteFixture(*dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "dpdfixture: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(dbPath)
	fmt.Println(tarPath)
}
