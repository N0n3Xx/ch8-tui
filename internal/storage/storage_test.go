package storage

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStoreRejectsUnsafeChatIDs(t *testing.T) {
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	for _, id := range []string{"../outside", "..", ".", "/tmp/chat", "nested/chat"} {
		if err := store.Save(&Chat{ID: id}); err == nil {
			t.Fatalf("Save(%q) succeeded, want error", id)
		}
		if _, err := store.Load(id); err == nil {
			t.Fatalf("Load(%q) succeeded, want error", id)
		}
		if err := store.Delete(id); err == nil {
			t.Fatalf("Delete(%q) succeeded, want error", id)
		}
	}
}

func TestStoreListUsesSafeFilenameID(t *testing.T) {
	dir := t.TempDir()
	data := []byte(`{"id":"../outside","title":"Injected","created_at":"2026-04-27T00:00:00Z","updated_at":"2026-04-27T00:00:00Z","messages":[]}`)
	if err := os.WriteFile(filepath.Join(dir, "safe-id.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	store, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	chats, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(chats) != 1 {
		t.Fatalf("len(chats) = %d, want 1", len(chats))
	}
	if chats[0].ID != "safe-id" {
		t.Fatalf("chat ID = %q, want filename ID", chats[0].ID)
	}
}

func TestStoreIgnoresUnsafeFilenames(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".json"), []byte(`{"messages":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ok.json"), []byte(`{"messages":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	store, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	chats, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(chats) != 1 || chats[0].ID != "ok" {
		t.Fatalf("chats = %#v, want only ok", chats)
	}
}

func TestStoreDeleteMissingSafeIDIsNoop(t *testing.T) {
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Delete("missing"); err != nil {
		t.Fatalf("Delete missing safe ID returned %v", err)
	}
}

func TestStoreSaveRejectsTraversalWithoutWritingOutsideDir(t *testing.T) {
	parent := t.TempDir()
	dir := filepath.Join(parent, "chats")
	store, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	err = store.Save(&Chat{
		ID:        "../outside",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	})
	if err == nil {
		t.Fatal("Save traversal ID succeeded, want error")
	}
	if _, statErr := os.Stat(filepath.Join(parent, "outside.json")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("outside file stat err = %v, want not exist", statErr)
	}
}
