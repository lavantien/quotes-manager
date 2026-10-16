package cli

import (
	"context"
	"fmt"

	"github.com/lavantien/quotes-manager/internal/config"
)

func init() {
	Register(Command{
		Name:  "version",
		Usage: "print version and schema version",
		Run: func(_ context.Context, _ *config.Config, _ []string) error {
			fmt.Printf("qm %s (schema %d)\n", config.Version, config.SchemaVersion)
			return nil
		},
	})
}
