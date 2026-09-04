package migrate

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	"github.com/ibednov/go-lepsios/files"
)

type DatabaseDSN struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
	SSLMode  string
}

type PendingMigration struct {
	Version int64
	Source  string
}

type BackupInput struct {
	Adapter          files.Adapter
	DB               DatabaseDSN
	Prefix           string
	StorageTypeLabel string
	CurrentVersion   int64
	Pending          []PendingMigration
}

func BackupBeforeMigrate(ctx context.Context, in BackupInput) error {
	if in.Adapter == nil {
		return fmt.Errorf("adapter is required")
	}

	prefix := strings.Trim(strings.TrimSpace(in.Prefix), "/")
	if prefix == "" {
		prefix = "backups/pre-migrate"
	}

	stamp := time.Now().UTC().Format("20060102T150405Z")
	objectPath := path.Join(prefix, stamp+"-"+in.DB.Name+".sql.gz")

	fmt.Printf(
		"migrate backup: pending=%d current_version=%d storage=%s path=%s\n",
		len(in.Pending),
		in.CurrentVersion,
		in.StorageTypeLabel,
		objectPath,
	)

	pr, pw := io.Pipe()
	errCh := make(chan error, 1)
	go func() {
		errCh <- streamPgDumpGzip(ctx, in.DB, pw)
	}()

	createErr := in.Adapter.Create(ctx, files.CreateInput{
		Path:   objectPath,
		Reader: pr,
	})
	dumpErr := <-errCh

	if createErr != nil {
		return fmt.Errorf("upload backup to file storage: %w", createErr)
	}
	if dumpErr != nil {
		_ = in.Adapter.Delete(ctx, files.DeleteInput{Path: objectPath})
		return fmt.Errorf("pg_dump: %w", dumpErr)
	}

	metaPath := objectPath + ".meta.txt"
	meta := fmt.Sprintf(
		"created_at_utc=%s\ndb=%s\nstorage_type=%s\ncurrent_version=%d\npending_count=%d\npending=\n%s\n",
		stamp,
		in.DB.Name,
		in.StorageTypeLabel,
		in.CurrentVersion,
		len(in.Pending),
		FormatPending(in.Pending),
	)
	if err := in.Adapter.Create(ctx, files.CreateInput{
		Path:   metaPath,
		Reader: strings.NewReader(meta),
	}); err != nil {
		return fmt.Errorf("upload backup meta: %w", err)
	}

	fmt.Printf("migrate backup: OK %s (+ %s)\n", objectPath, metaPath)
	return nil
}

func streamPgDumpGzip(ctx context.Context, db DatabaseDSN, w *io.PipeWriter) (err error) {
	defer func() {
		if err != nil {
			_ = w.CloseWithError(err)
			return
		}
		_ = w.Close()
	}()

	pgDump, lookErr := exec.LookPath("pg_dump")
	if lookErr != nil {
		return fmt.Errorf("pg_dump not found in PATH (install postgresql-client in image): %w", lookErr)
	}

	cmd := exec.CommandContext(ctx, pgDump,
		"--host", db.Host,
		"--port", db.Port,
		"--username", db.User,
		"--dbname", db.Name,
		"--no-owner",
		"--no-acl",
		"--format=plain",
	)
	cmd.Env = append(os.Environ(),
		"PGPASSWORD="+db.Password,
		"PGSSLMODE="+db.SSLMode,
	)
	cmd.Stderr = os.Stderr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}

	gw := gzip.NewWriter(w)
	if _, err := io.Copy(gw, stdout); err != nil {
		_ = cmd.Process.Kill()
		_ = gw.Close()
		_ = cmd.Wait()
		return fmt.Errorf("copy pg_dump output: %w", err)
	}
	if err := gw.Close(); err != nil {
		_ = cmd.Wait()
		return fmt.Errorf("gzip close: %w", err)
	}
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("pg_dump failed: %w", err)
	}
	return nil
}

func FormatPending(pending []PendingMigration) string {
	if len(pending) == 0 {
		return "(none)"
	}
	var b strings.Builder
	for _, m := range pending {
		b.WriteString(fmt.Sprintf("- %d %s\n", m.Version, path.Base(m.Source)))
	}
	return b.String()
}
