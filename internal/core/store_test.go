package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func testStore(t *testing.T) (*Store, string) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "data")
	s, err := OpenStore(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s, dir
}

func TestStoreMigration(t *testing.T) {
	ctx := context.Background()
	s, dir := testStore(t)
	var mode string
	var version, foreignKeys int
	if err := s.db.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil || mode != "wal" {
		t.Fatalf("WAL: %q %v", mode, err)
	}
	if err := s.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil || version != schemaVersion {
		t.Fatalf("schema: %d %v", version, err)
	}
	if err := s.db.QueryRow("PRAGMA foreign_keys").Scan(&foreignKeys); err != nil || foreignKeys != 1 {
		t.Fatalf("foreign keys: %d %v", foreignKeys, err)
	}
	if err := s.InitAdmin(ctx, "test-password-long"); err != nil {
		t.Fatal(err)
	}
	if err := s.InitAdmin(ctx, "second-password"); err == nil {
		t.Fatal("admin overwrite accepted")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenStore(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	var count int
	if err = reopened.db.QueryRow("SELECT count(*) FROM operators").Scan(&count); err != nil || count != 1 {
		t.Fatalf("persistence: %d %v", count, err)
	}
	if _, err = reopened.db.Exec("PRAGMA user_version=999"); err != nil {
		t.Fatal(err)
	}
	reopened.Close()
	if invalid, err := OpenStore(ctx, dir); err == nil {
		invalid.Close()
		t.Fatal("future schema accepted")
	}
}

func TestStoreRejectsUnsafeDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "unsafe")
	if err := os.Mkdir(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if s, err := OpenStore(context.Background(), dir); err == nil {
		s.Close()
		t.Fatal("public data directory accepted")
	}
}
