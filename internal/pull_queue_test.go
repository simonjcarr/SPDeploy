package internal

import (
	"testing"
	"time"
)

func TestPullQueue(t *testing.T) {
	pq := NewPullQueue()

	repo1 := Repository{
		URL:    "https://github.com/test/repo1",
		Branch: "main",
		Path:   "/tmp/repo1",
	}

	repo2 := Repository{
		URL:    "https://github.com/test/repo2",
		Branch: "main",
		Path:   "/tmp/repo2",
	}

	// Test adding to queue
	pq.Add(repo1, nil)
	if pq.Size() != 1 {
		t.Errorf("Expected queue size 1, got %d", pq.Size())
	}

	// Test duplicate prevention
	pq.Add(repo1, nil)
	if pq.Size() != 1 {
		t.Errorf("Expected queue size 1 after duplicate add, got %d", pq.Size())
	}

	// Test adding different repo
	pq.Add(repo2, nil)
	if pq.Size() != 2 {
		t.Errorf("Expected queue size 2, got %d", pq.Size())
	}

	// Test processing flag
	if pq.IsProcessing() {
		t.Error("Expected processing to be false initially")
	}

	// Test getting next item
	pr, ok := pq.GetNext()
	if !ok {
		t.Error("Expected to get next item from queue")
	}
	if pr.Repo.URL != repo1.URL {
		t.Errorf("Expected first repo URL %s, got %s", repo1.URL, pr.Repo.URL)
	}
	if !pq.IsProcessing() {
		t.Error("Expected processing to be true after GetNext")
	}

	// Test that GetNext blocks when processing
	done := make(chan bool)
	go func() {
		_, _ = pq.GetNext()
		done <- true
	}()

	select {
	case <-done:
		t.Error("GetNext should have blocked while processing")
	case <-time.After(100 * time.Millisecond):
		// Expected behavior - GetNext is blocking
	}

	// Release processing flag
	pq.SetProcessing(false)

	// Now GetNext should proceed
	select {
	case <-done:
		// Expected behavior - GetNext unblocked
	case <-time.After(100 * time.Millisecond):
		t.Error("GetNext should have unblocked after SetProcessing(false)")
	}
}