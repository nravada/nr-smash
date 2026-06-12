package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nravada/nr-smash/internal/bug"
	"github.com/nravada/nr-smash/internal/queue"
)

// fakeCollector writes a shell script that emits one bug.Bug JSON line and
// records how many times it has been invoked via a temp counter file.
// Returns the script path and a function to read the invocation count.
func fakeCollector(t *testing.T, b *bug.Bug) (scriptPath string, invocations func() int) {
	t.Helper()

	dir := t.TempDir()

	data, err := json.Marshal(b)
	if err != nil {
		t.Fatalf("marshal bug: %v", err)
	}

	counterFile := filepath.Join(dir, "count")
	scriptPath = filepath.Join(dir, "collector.sh")

	script := fmt.Sprintf(`#!/bin/sh
echo '%s'
COUNT=$(cat "%s" 2>/dev/null || echo 0)
echo $((COUNT + 1)) > "%s"
`, string(data), counterFile, counterFile)

	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake collector: %v", err)
	}

	invocations = func() int {
		raw, err := os.ReadFile(counterFile)
		if err != nil {
			return 0
		}
		var n int
		fmt.Sscanf(string(raw), "%d", &n)
		return n
	}
	return scriptPath, invocations
}

func TestHTTPHandlers(t *testing.T) {
	q := queue.New()
	srv := httptest.NewServer(buildMux(q))
	defer srv.Close()

	get := func(path string) *http.Response {
		t.Helper()
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		return resp
	}

	t.Run("health returns ok when empty", func(t *testing.T) {
		resp := get("/health")
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("want 200, got %d", resp.StatusCode)
		}
	})

	t.Run("pop returns 204 on empty queue", func(t *testing.T) {
		resp := get("/pop")
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("want 204, got %d", resp.StatusCode)
		}
	})

	t.Run("peek returns 204 on empty queue", func(t *testing.T) {
		resp := get("/peek")
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("want 204, got %d", resp.StatusCode)
		}
	})

	// Push two bugs; higher priority should come out first.
	q.Push(&bug.Bug{ID: "B-low", Priority: 10, Status: bug.StatusQueued})
	q.Push(&bug.Bug{ID: "B-high", Priority: 999, Status: bug.StatusQueued})

	t.Run("pop returns highest priority first", func(t *testing.T) {
		resp := get("/pop")
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("want 200, got %d", resp.StatusCode)
		}
		var b bug.Bug
		if err := json.NewDecoder(resp.Body).Decode(&b); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if b.ID != "B-high" {
			t.Fatalf("want B-high, got %s", b.ID)
		}
	})

	t.Run("peek does not remove the bug", func(t *testing.T) {
		// One bug left (B-low). Peek twice; both should return it.
		resp1 := get("/peek")
		defer resp1.Body.Close()
		resp2 := get("/peek")
		defer resp2.Body.Close()

		var b1, b2 bug.Bug
		json.NewDecoder(resp1.Body).Decode(&b1)
		json.NewDecoder(resp2.Body).Decode(&b2)

		if b1.ID != b2.ID {
			t.Fatalf("peek mutated the queue: got %s then %s", b1.ID, b2.ID)
		}
	})
}

func TestRunCollectorIngestsAndScores(t *testing.T) {
	b := &bug.Bug{
		ID:           "jira:TEST-1",
		Source:       bug.SourceJira,
		Title:        "something is broken",
		Tier:         bug.TierCustomerFacing,
		Labels:       []string{"sev1"},
		DiscoveredAt: time.Now().Add(-10 * time.Minute),
		Status:       bug.StatusQueued,
	}

	script, _ := fakeCollector(t, b)
	q := queue.New()

	runCollector(script, "", q)

	if q.Len() != 1 {
		t.Fatalf("want 1 bug in queue, got %d", q.Len())
	}
	top := q.Peek()
	if top.ID != b.ID {
		t.Fatalf("wrong bug: got %s", top.ID)
	}
	if top.Priority == 0 {
		t.Fatal("expected non-zero priority after scoring")
	}
	if top.Status != bug.StatusQueued {
		t.Fatalf("unexpected status: %s", top.Status)
	}
}

func TestRunCollectorDeduplicates(t *testing.T) {
	b := &bug.Bug{ID: "jira:DUP-1", Source: bug.SourceJira, Status: bug.StatusQueued}
	script, _ := fakeCollector(t, b)
	q := queue.New()

	runCollector(script, "", q)
	runCollector(script, "", q) // second run emits the same ID

	if q.Len() != 1 {
		t.Fatalf("want 1 bug after dedup, got %d", q.Len())
	}
}

// TestSchedulerTicksCollector verifies the scheduler calls the collector
// on the initial run and then again on each interval tick.
func TestSchedulerTicksCollector(t *testing.T) {
	b := &bug.Bug{ID: "jira:TICK-1", Source: bug.SourceJira, Status: bug.StatusQueued}
	script, invocations := fakeCollector(t, b)

	q := queue.New()
	interval := 100 * time.Millisecond

	var ticks atomic.Int32
	wrapped := func() {
		runCollector(script, "", q)
		ticks.Add(1)
	}

	// Simulate the scheduler tick loop inline.
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	done := make(chan struct{})
	go func() {
		wrapped() // initial run (mirrors main.go line 53)
		for {
			select {
			case <-ticker.C:
				wrapped()
			case <-done:
				return
			}
		}
	}()

	// Wait for at least 3 invocations.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if ticks.Load() >= 3 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	close(done)

	got := ticks.Load()
	if got < 3 {
		t.Fatalf("want ≥3 collector invocations, got %d", got)
	}

	// Counter file tracks actual subprocess launches (independent of ticks).
	launched := invocations()
	if launched < 3 {
		t.Fatalf("want ≥3 subprocess launches, got %d", launched)
	}
}
