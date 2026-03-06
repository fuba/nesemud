package streaming

import "testing"

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
