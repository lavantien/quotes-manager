package server

import (
	"io"
	"log/slog"
	"testing"

	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	goleak.VerifyTestMain(m)
}
