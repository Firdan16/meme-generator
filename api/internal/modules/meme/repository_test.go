package meme

import (
	"testing"

	"github.com/nyxsr/meme-portal/api/internal/testutil"
)

func TestMemeOwnerCanReadUpdateAndDelete(t *testing.T) {
	pool := testutil.Postgres(t)
	userID := testutil.User(t, pool)
	repo := NewPostgresRepository(pool)
	ctx := t.Context()
	created, err := repo.Create(ctx, Meme{UserID: userID, Title: "fixture", BaseImageURL: "base/test.png"})
	if err != nil {
		t.Fatal(err)
	}
	got, err := repo.GetByID(ctx, created.ID, userID)
	if err != nil || got.ID != created.ID || got.UserID != userID {
		t.Fatalf("owner read failed: %v", err)
	}
	updated, err := repo.SetResultURL(ctx, created.ID, userID, "result/test.png")
	if err != nil || updated.ResultURL == nil || *updated.ResultURL != "result/test.png" {
		t.Fatalf("owner result update failed: %v", err)
	}
	if err := repo.Delete(ctx, created.ID, userID); err != nil {
		t.Fatalf("owner delete failed: %v", err)
	}
}
