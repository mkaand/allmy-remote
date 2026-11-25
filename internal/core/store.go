package core

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

const schemaVersion = 2

type Store struct {
	db *sql.DB
}

func OpenStore(ctx context.Context, dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return nil, errors.New("data directory must be private (0700)")
	}
	path := filepath.Join(dir, "core.sqlite")
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
	if err == nil {
		if err = f.Close(); err != nil {
			return nil, err
		}
	} else if !errors.Is(err, os.ErrExist) {
		return nil, err
	}
	info, err = os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		return nil, errors.New("database must be a private regular file (0600)")
	}
	// Reject URI interpretation of configured paths.
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", absolute)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	s := &Store{db: db}
	if err = s.initialize(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) initialize(ctx context.Context) error {
	var mode string
	if err := s.db.QueryRowContext(ctx, "PRAGMA journal_mode=WAL").Scan(&mode); err != nil || mode != "wal" {
		return errors.New("could not enable SQLite WAL")
	}
	for _, statement := range []string{"PRAGMA foreign_keys=ON", "PRAGMA busy_timeout=5000"} {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var version int
	if err = tx.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return err
	}
	switch version {
	case 0:
		for _, statement := range []string{
			"CREATE TABLE operators (id INTEGER PRIMARY KEY CHECK (id = 1), username TEXT NOT NULL UNIQUE CHECK (username = 'admin'), salt BLOB NOT NULL, password_hash BLOB NOT NULL, created_at INTEGER NOT NULL)",
			"CREATE TABLE operator_sessions (token_hash BLOB PRIMARY KEY NOT NULL, operator_id INTEGER NOT NULL REFERENCES operators(id) ON DELETE CASCADE, expires_at INTEGER NOT NULL)",
			"CREATE INDEX operator_sessions_expiry ON operator_sessions(expires_at)",
			"PRAGMA user_version=1",
		} {
			if _, err = tx.ExecContext(ctx, statement); err != nil {
				return err
			}
		}
		fallthrough
	case 1:
		for _, statement := range []string{
			"CREATE TABLE enrollment_grants (token_hash BLOB PRIMARY KEY NOT NULL, name TEXT NOT NULL, expires_at INTEGER NOT NULL)",
			"CREATE TABLE devices (id TEXT PRIMARY KEY NOT NULL, name TEXT NOT NULL, credential_hash BLOB UNIQUE NOT NULL, created_at INTEGER NOT NULL, last_seen INTEGER, revoked_at INTEGER)",
			"PRAGMA user_version=2",
		} {
			if _, err = tx.ExecContext(ctx, statement); err != nil {
				return err
			}
		}
	case schemaVersion:
	default:
		return fmt.Errorf("unsupported schema version %d", version)
	}
	return tx.Commit()
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) Ready() bool {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	return s.db.PingContext(ctx) == nil
}

func (s *Store) InitAdmin(ctx context.Context, password string) error {
	if len(password) < 12 || len(password) > 256 {
		return errors.New("password must be 12–256 bytes")
	}
	salt, err := randomBytes(16)
	if err != nil {
		return err
	}
	hash := passwordHash(password, salt)
	result, err := s.db.ExecContext(ctx, "INSERT OR IGNORE INTO operators(id, username, salt, password_hash, created_at) VALUES(1, 'admin', ?, ?, ?)", salt, hash, time.Now().Unix())
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return errors.New("administrator already initialized")
	}
	return nil
}
