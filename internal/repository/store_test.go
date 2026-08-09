package repository_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"kohame/internal/config"
	"kohame/internal/database"
	"kohame/internal/repository"
)

func TestRenameMovesRepositoryAndSettings(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	db, err := database.Open(config.DatabaseConfig{Driver: "sqlite", DSN: filepath.Join(root, "kohame.db")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	store := repository.NewStore(filepath.Join(root, "repos"), db, "sqlite")
	ctx := context.Background()
	repo, err := store.Create(ctx, "alice", "before")
	if err != nil {
		t.Fatal(err)
	}
	settings, err := store.Settings(ctx, repo.FullName)
	if err != nil {
		t.Fatal(err)
	}
	settings.Description = "Repository used by the rename test"
	settings.HomepageURL = "https://example.com/kohame"
	settings.AllowForks = false
	if err := store.UpdateSettings(ctx, repo.FullName, settings); err != nil {
		t.Fatal(err)
	}

	renamed, err := store.Rename(ctx, repo, "after")
	if err != nil {
		t.Fatal(err)
	}
	if renamed.FullName != "alice/after" {
		t.Fatalf("renamed repository = %q, want alice/after", renamed.FullName)
	}
	if _, err := store.Get("alice", "before"); !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("old repository lookup error = %v, want ErrNotFound", err)
	}
	updated, err := store.Settings(ctx, renamed.FullName)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Description != settings.Description || updated.HomepageURL != settings.HomepageURL || updated.AllowForks != settings.AllowForks {
		t.Fatalf("settings after rename = %#v, want preserved settings %#v", updated, settings)
	}
}
