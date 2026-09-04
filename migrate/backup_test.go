package migrate_test

import (
	"path"
	"strings"
	"testing"

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
