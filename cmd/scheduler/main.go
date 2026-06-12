// scheduler — periodically launches the bug collector, scores its output,
// and exposes a priority-ordered queue over HTTP for the resolver.
//
// HTTP API (all JSON):
//
//	GET  /pop      — dequeue and return the highest-priority bug (204 if empty)
//	GET  /peek     — inspect the top bug without removing it (204 if empty)
//	GET  /queue    — list all queued bugs (for debugging)
//	GET  /health   — liveness check
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/nravada/nr-smash/internal/bug"
	"github.com/nravada/nr-smash/internal/priority"
	"github.com/nravada/nr-smash/internal/queue"
)

func main() {
	var (
		interval     = flag.Duration("interval", 5*time.Minute, "how often to run the collector")
		listen       = flag.String("listen", ":7777", "address to serve the queue API on")
		collectorBin = flag.String("collector-bin", "./collector", "path to the collector binary")
		extraFlags   = flag.String("collector-flags", "", "extra flags forwarded to the collector (space-separated)")
	)
	flag.Parse()

	q := queue.New()

	// Start the HTTP queue server in the background.
	srv := &http.Server{Addr: *listen, Handler: buildMux(q)}
	go func() {
		log.Printf("scheduler: queue API listening on %s", *listen)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("scheduler: HTTP server error: %v", err)
		}
	}()

	// Run the collector once immediately, then on every tick.
	runCollector(*collectorBin, *extraFlags, q)

	ticker := time.NewTicker(*interval)
	defer ticker.Stop()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	log.Printf("scheduler: running collector every %s", *interval)
	for {
		select {
		case <-ticker.C:
			runCollector(*collectorBin, *extraFlags, q)
		case <-stop:
			log.Println("scheduler: shutting down")
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = srv.Shutdown(ctx)
			return
		}
	}
}

// runCollector launches the collector binary, reads newline-delimited JSON
// bug.Bug records from its stdout, scores each one, and pushes it onto q.
func runCollector(bin, extraFlags string, q *queue.Queue) {
	args := []string{}
	if extraFlags != "" {
		args = strings.Fields(extraFlags)
	}

	cmd := exec.Command(bin, args...) //nolint:gosec
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Printf("scheduler: collector pipe error: %v", err)
		return
	}
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		log.Printf("scheduler: failed to start collector (%s): %v", bin, err)
		return
	}

	now := time.Now()
	ingested := 0
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var b bug.Bug
		if err := json.Unmarshal(line, &b); err != nil {
			log.Printf("scheduler: skipping non-JSON line from collector: %s", line)
			continue
		}
		b.Priority = priority.Score(&b, now)
		if b.Status == "" {
			b.Status = bug.StatusQueued
		}
		q.Push(&b)
		ingested++
	}
	if err := scanner.Err(); err != nil {
		log.Printf("scheduler: collector stdout read error: %v", err)
	}

	if err := cmd.Wait(); err != nil {
		log.Printf("scheduler: collector exited with error: %v", err)
	}

	log.Printf("scheduler: collector run complete — ingested %d bugs, queue depth %d", ingested, q.Len())
}

func buildMux(q *queue.Queue) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"status":"ok","queue_depth":%d}`, q.Len())
	})

	mux.HandleFunc("/pop", func(w http.ResponseWriter, r *http.Request) {
		b := q.Pop()
		if b == nil {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(b)
	})

	mux.HandleFunc("/peek", func(w http.ResponseWriter, r *http.Request) {
		b := q.Peek()
		if b == nil {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(b)
	})

	mux.HandleFunc("/queue", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(q.Snapshot())
	})

	return mux
}
