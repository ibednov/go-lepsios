package migrate_test

import (
	"context"
	"errors"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ibednov/go-lepsios/files"
	"github.com/ibednov/go-lepsios/migrate"
)

func TestFormatPending(t *testing.T) {
	t.Parallel()

	if got := migrate.FormatPending(nil); got != "(none)" {
		t.Fatalf("nil pending: got %q", got)
	}

	pending := []migrate.PendingMigration{
		{Version: 20260816194500, Source: "/root/migrations/20260816194500_create_admin_action_events.sql"},
	}
	got := migrate.FormatPending(pending)
	if !strings.Contains(got, "20260816194500") {
		t.Fatalf("missing version in %q", got)
	}
	if !strings.Contains(got, path.Base(pending[0].Source)) {
		t.Fatalf("missing file name in %q", got)
	}
}

type failingUploadAdapter struct{ err error }

func (a failingUploadAdapter) Create(context.Context, files.CreateInput) error { return a.err }
func (failingUploadAdapter) Get(context.Context, files.GetInput) (io.ReadCloser, error) {
	return nil, errors.New("not implemented")
}
func (failingUploadAdapter) Delete(context.Context, files.DeleteInput) error { return nil }
func (failingUploadAdapter) PublicURL(string) string                         { return "" }

func TestBackupBeforeMigrateUnblocksDumpWhenUploadFails(t *testing.T) {
	pgDumpDir := t.TempDir()
	pgDump := filepath.Join(pgDumpDir, "pg_dump")
	script := "#!/bin/sh\ni=0\nwhile [ \"$i\" -lt 256 ]; do\n  printf '%4096s' x\n  i=$((i + 1))\ndone\n"
	if err := os.WriteFile(pgDump, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", pgDumpDir)

	uploadErr := errors.New("simulated object-store failure")
	done := make(chan error, 1)
	go func() {
		done <- migrate.BackupBeforeMigrate(context.Background(), migrate.BackupInput{
			Adapter: failingUploadAdapter{err: uploadErr},
			DB:      migrate.DatabaseDSN{Host: "db", Port: "5432", User: "test", Name: "test"},
		})
	}()

	select {
	case err := <-done:
		if !errors.Is(err, uploadErr) {
			t.Fatalf("expected upload error, got %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("backup hung after object-store upload failed")
	}
}
