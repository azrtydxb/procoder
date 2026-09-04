package api

import (
	"io"
	"sync"
	"testing"
	"time"
)

// Requests against one repository run one at a time, and requests against
// different repositories do not wait for each other.
//
// proved by: dropping queues.do from serveConn — the overlap counter goes
// above one, which in production is store.Lock timing out after five
// seconds and answering "the write was NOT made" for a lock the same
// process holds.
func TestPerRootSerialisation(t *testing.T) {
	var mu sync.Mutex
	inside, maxInside := 0, 0

	path, _ := testServer(t, func(req Request, stdout, stderr io.Writer) (int, *Result) {
		mu.Lock()
		inside++
		if inside > maxInside {
			maxInside = inside
		}
		mu.Unlock()

		// Long enough that overlap would be certain without the queue.
		time.Sleep(2 * time.Millisecond)

		mu.Lock()
		inside--
		mu.Unlock()
		return 0, nil
	})

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := (Client{Path: path}).Do(Request{Argv: []string{"claims"}, Cwd: "/one"}); err != nil {
				t.Errorf("request failed: %v", err)
			}
		}()
	}
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	if maxInside > 1 {
		t.Fatalf("%d requests against one repository ran at once — the store cannot survive that", maxInside)
	}
}

// Two repositories are two queues. Serialising them together would make
// the daemon slower than the spawning it replaces.
func TestDifferentRepositoriesDoNotWait(t *testing.T) {
	var q queues
	started := make(chan struct{})
	release := make(chan struct{})
	done := make(chan struct{})

	go q.do("one", func() {
		close(started)
		<-release
	})
	<-started

	go q.do("two", func() { close(done) })

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("a request for one repository waited on another repository's")
	}
	close(release)
}
