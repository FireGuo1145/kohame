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

func TestInitializeCreatesReadmeAndDetectableLicense(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	db, err := database.Open(config.DatabaseConfig{Driver: "sqlite", DSN: filepath.Join(root, "kohame.db")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	store := repository.NewStore(filepath.Join(root, "repos"), db, "sqlite")
	repo, err := store.Create(context.Background(), "alice", "starter")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Initialize(context.Background(), repo, "alice", "alice@example.com", true, "MIT"); err != nil {
		t.Fatal(err)
	}
	readme, err := store.Blob(context.Background(), repo, "main", "README.md")
	if err != nil || !readme.IsText {
		t.Fatalf("README lookup = %#v, %v", readme, err)
	}
	if license := store.DetectLicense(context.Background(), repo, "main"); license != "MIT" {
		t.Fatalf("detected license = %q, want MIT", license)
	}
}

func TestMergePullRequestBranch(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	db, err := database.Open(config.DatabaseConfig{Driver: "sqlite", DSN: filepath.Join(root, "kohame.db")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	store := repository.NewStore(filepath.Join(root, "repos"), db, "sqlite")
	ctx := context.Background()
	repo, err := store.Create(ctx, "alice", "mergeable")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Initialize(ctx, repo, "alice", "alice@example.com", true, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := store.WriteFile(ctx, repo, "feature", "feature.txt", "hello from feature\n", "alice", "alice@example.com", "add feature"); err != nil {
		t.Fatal(err)
	}
	files, err := store.PullRequestDiff(ctx, repo, "feature", "main")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Path != "feature.txt" {
		t.Fatalf("pull request files = %#v", files)
	}
	if _, err := store.MergePullRequest(ctx, repo, "feature", "main", "alice", "alice@example.com"); err != nil {
		t.Fatal(err)
	}
	file, err := store.Blob(ctx, repo, "main", "feature.txt")
	if err != nil || file.Content != "hello from feature\n" {
		t.Fatalf("merged file = %#v, %v", file, err)
	}
}

func TestWikiPagesPersistAcrossRename(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	db, err := database.Open(config.DatabaseConfig{Driver: "sqlite", DSN: filepath.Join(root, "kohame.db")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	store := repository.NewStore(filepath.Join(root, "repos"), db, "sqlite")
	ctx := context.Background()
	repo, err := store.Create(ctx, "alice", "wiki-source")
	if err != nil {
		t.Fatal(err)
	}
	created, err := store.SaveWikiPage(ctx, repo.FullName, repository.WikiPage{Slug: "getting-started", Title: "快速开始", Content: "# Hello", Author: "alice"})
	if err != nil {
		t.Fatal(err)
	}
	if created.Title != "快速开始" || created.Content != "# Hello" {
		t.Fatalf("created wiki page = %#v", created)
	}
	if _, err = store.SaveWikiPage(ctx, repo.FullName, repository.WikiPage{Slug: "getting-started", Title: "快速开始", Content: "# Updated", Author: "alice"}); err != nil {
		t.Fatal(err)
	}
	history, err := store.WikiHistory(ctx, repo.FullName, "getting-started")
	if err != nil || len(history) != 2 {
		t.Fatalf("wiki history = %#v, %v", history, err)
	}
	renamed, err := store.Rename(ctx, repo, "wiki-target")
	if err != nil {
		t.Fatal(err)
	}
	stored, err := store.WikiPage(ctx, renamed.FullName, "getting-started")
	if err != nil || stored.Content != "# Updated" {
		t.Fatalf("renamed wiki page = %#v, %v", stored, err)
	}
	if err = store.DeleteWikiPage(ctx, renamed.FullName, "getting-started"); err != nil {
		t.Fatal(err)
	}
	if _, err = store.WikiPage(ctx, renamed.FullName, "getting-started"); !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("deleted wiki page lookup error = %v, want ErrNotFound", err)
	}
}
