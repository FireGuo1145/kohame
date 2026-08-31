package forge

import (
	"context"
	"path/filepath"
	"testing"

	"kohame/internal/config"
	"kohame/internal/database"
)

func TestPullRequestReviewsTrackLatestReviewerDecision(t *testing.T) {
	root := t.TempDir()
	gormDB, err := database.Open(config.DatabaseConfig{Driver: "sqlite", DSN: filepath.Join(root, "kohame.db")})
	if err != nil {
		t.Fatal(err)
	}
	db, err := database.SQLDB(gormDB)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	store := NewStore(db, "sqlite")
	ctx := context.Background()
	author, err := store.CreateAdmin(ctx, "alice", "alice@example.com", "password-1")
	if err != nil {
		t.Fatal(err)
	}
	reviewer, err := store.Register(ctx, "bob", "bob@example.com", "password-2")
	if err != nil {
		t.Fatal(err)
	}
	pull, err := store.CreatePullRequest(ctx, "alice/project", author, "Improve docs", "", "docs", "main")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.SavePullRequestReview(ctx, "alice/project", pull.ID, author, "approved", ""); err == nil {
		t.Fatal("author approval succeeded")
	}
	if _, err := store.SavePullRequestReview(ctx, "alice/project", pull.ID, reviewer, "approved", "looks good"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SavePullRequestReview(ctx, "alice/project", pull.ID, reviewer, "changes_requested", "please revise"); err != nil {
		t.Fatal(err)
	}
	count, err := store.PullRequestApprovalCount(ctx, "alice/project", pull.ID)
	if err != nil || count != 0 {
		t.Fatalf("approval count = %d, %v; want 0", count, err)
	}
	reviews, err := store.PullRequestReviews(ctx, "alice/project", pull.ID)
	if err != nil || len(reviews) != 1 || reviews[0].State != "changes_requested" {
		t.Fatalf("reviews = %#v, %v", reviews, err)
	}
}
