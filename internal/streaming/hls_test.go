package streaming

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestEnqueueLatestWhenSpaceAvailable(t *testing.T) {
	ch := make(chan []byte, 2)
	dropped := enqueueLatest(ch, []byte{1, 2, 3})
	if dropped != 0 {
		t.Fatalf("expected dropped=0 got=%d", dropped)
	}
	got := <-ch
	if len(got) != 3 || got[0] != 1 || got[1] != 2 || got[2] != 3 {
		t.Fatalf("unexpected payload: %v", got)
	}
}

func TestEnqueueLatestDropsOldestWhenFull(t *testing.T) {
	ch := make(chan []byte, 1)
	ch <- []byte{1}
	dropped := enqueueLatest(ch, []byte{2})
	if dropped != 1 {
		t.Fatalf("expected dropped=1 got=%d", dropped)
	}
	got := <-ch
	if len(got) != 1 || got[0] != 2 {
		t.Fatalf("expected latest payload kept, got=%v", got)
	}
}

func TestClearHLSOutputDirRemovesOnlyPlaylistArtifacts(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"index.m3u8", "index1.ts", "keep.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	if err := clearHLSOutputDir(dir); err != nil {
		t.Fatalf("clearHLSOutputDir: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "index.m3u8")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("index.m3u8 should be removed, err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "index1.ts")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("index1.ts should be removed, err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "keep.txt")); err != nil {
		t.Fatalf("keep.txt should remain, err=%v", err)
	}
}
