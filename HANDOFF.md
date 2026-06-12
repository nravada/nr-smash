# Handoff: nr-smash agent memory setup

Bridge between Claude Code sessions. **Last updated:** post-MCP probe + design ratified. **This handoff is intended for a teammate's fresh Claude Code session — they clone the repo and pick up Phase 3 implementation cold.**

---

## 0. TL;DR (read first if cold)

Continuing **nr-smash**, Go scheduled SRE agent. Phase 3 (`memory.Store` impl) design is **locked**, MCP **probed live**, real tool shapes captured. **Implementation pending — that's your job, teammate.**

**Status of design decisions (locked):**
- Short-circuit-only flow (no hypothesis tier, no human gate). On a high-similarity memory hit, resolver emits past `FixShape` + `PRURL` and exits with `Bug.Status=resolved-via-memory`. No clone/fix/test. See §7.6.
- Threshold lives in `defaults.short_circuit_threshold` (Phase 1 config). Placeholder `0.95`. **Calibration deferred (§13.1) — DO THIS BEFORE LOCKING.**
- Search/short-circuit fires **resolver-side** (not triage).
- Output on short-circuit: `.smash/<smash-id>/short-circuit.md` containing matched Entry + score. New status enum value `resolved-via-memory` (§15) — needs `[contract]` PR.
- MCP tool surface: **probed live this session**. Real shapes in §7.2. Old guesses (`memory_get`, `score`, `matches`, 384-dim) are wrong; corrected throughout.

**Status of open items:**
- [OPEN] Threshold calibration battery (§13.1) — placeholder 0.95 confirmed too high; near-paraphrase scored 0.6366 in probe.
- [OPEN] Commit `.mcp.json` and this `HANDOFF.md` to `main` before teammate clones — both are currently untracked (§17).
- [OPEN] Phase 2 (SQLite ops state) — scaffold says skip, handoff recommends do; user to decide before Phase 5.

**Cold-start action sequence (for teammate):**
1. Repo cloned, on `main`.
2. Open Claude Code at repo root → accept project-scope MCP server (`.mcp.json`) prompt.
3. `claude mcp list` → expect `claude-flow ✓ Connected`. (§6.1)
4. Read this file end-to-end. §0–§7 + §13–§16 are load-bearing.
5. Run threshold calibration battery (§13.1) — ~5 min, decides final threshold.
6. Implement Phase 3 per §7. Tool shapes already captured live; don't re-probe unless surprised.
7. Then Phase 4 (markdown), Phase 1 (config), Phase 5 (wiring), Phase 6 (tests).

**Execution order: 3 → 4 → 1 → (2?) → 5 → 6** (phase numbers stable for tasks; order is what matters).

---

## 1. Project context

- **Repo:** `/Users/aryansingh/Documents/hackathon2k26/nr-smash`
- **Branch:** `main` (only branch)
- **Stack:** Go 1.25.6, module `github.com/nravada/nr-smash`
- **Deps:** none yet. Will add `gopkg.in/yaml.v3` (Phase 1), maybe `modernc.org/sqlite` (Phase 2, pure Go).
- **Mission (README):** SMASH = Semantic Multi-source Autonomous SRE Hub. Always-on agent: triages bugs from team repos / Jira / Slack help channel → drives each to draft PR.
- **Scope:** hackathon prototype. Out of scope per `ARCHITECTURE.md:132+`: production orchestration, multi-tenant, persistent queue, HA/failover.

### 1.1 Tree (annotated)

```
nr-smash/
├── ARCHITECTURE.md          # canonical flow + contracts (READ FIRST)
├── README.md, CONTRIBUTING.md, LICENSE (Apache-2.0)
├── HANDOFF.md               # this file
├── go.mod                   # bare, no deps
├── .mcp.json                # NEW, UNTRACKED — claude-flow MCP project-scope (commit recommended)
├── .github/{CODEOWNERS,pull_request_template.md,ISSUE_TEMPLATE/{bug,feature}.md}
│   # CODEOWNERS mostly TBD; resolver owned by @rmalhan-thatsit
├── cmd/                     # 5 entrypoints, all bare main.go stubs
│   ├── collector/   # poll Slack+Jira → bug.Bug
│   ├── triage/      # repo scan + tier classify
│   ├── scheduler/   # orchestrator, dedup, priority queue :7777
│   ├── resolver/    # work-doer: clone/fix/test/draft PR
│   └── search/      # CLI for human spot-check of memory
├── internal/                # shared contracts (DON'T change without [contract] PR)
│   ├── bug/{bug.go,bug_test.go}
│   ├── memory/memory.go     # Store INTERFACE + Entry struct, no impl
│   └── priority/{priority.go,priority_test.go}
└── docs/components/{bug-collector,triage-agent,scheduler,resolver,search}.md
```

### 1.2 What's there vs not

| File | State |
|---|---|
| `internal/memory/memory.go` | Interface + Entry only; no impl |
| `internal/{bug,priority}/*.go` | Assumed complete; not re-read this session |
| `cmd/*/main.go` | Bare stubs, need impl per `docs/components/*.md` |
| `ARCHITECTURE.md`, `docs/components/*.md` | Complete; checklists unchecked |
| `config/teams.yaml` | **Missing — Phase 1** |
| `playbooks/*.md` | **Missing — Phase 4** |
| `internal/memory/claudeflow.go` | **Missing — Phase 3 (next task)** |
| `.mcp.json` | Created this session, untracked |

### 1.3 Required env (per `docs/components/bug-collector.md`)

```
SLACK_BOT_TOKEN=xoxb-...
JIRA_BASE_URL=https://new-relic.atlassian.net
ANTHROPIC_API_KEY=sk-ant-...
```
Real `.env` gitignored; new vars need `.env.example` entry.

---

## 2. Scaffold vs implementation — critical orientation

Scaffold is contract + skeleton. Interfaces committed; impls empty. Things that *look* done but aren't:
- `memory.Store` — interface only, zero impls.
- All `cmd/*/main.go` — bare stubs.
- `*_test.go` — likely scaffold-shape; verify with `go test ./...`.
- ARCHITECTURE.md "memory backend = claude-flow MCP" is a recommendation, nothing implements it.

### 2.1 `memory.Entry` schema (verbatim from `internal/memory/memory.go`)

```go
type Entry struct {
    SmashID     string    // stable ID matching resolver branch + change-doc
    BugID       string    // originating Bug.ID
    Repo        string    // e.g. "newrelic/nri-mssql"
    Pattern     string    // categorical label — see §17 enum
    FixShape    string    // 1-paragraph fix description
    LOCDelta    [2]int    // [added, removed]
    Tier        string    // trivial|standard|architectural|customer-facing
    PRURL       string
    PassedTests bool
    ShippedAt   time.Time
    Embedding   []float32 // 384-dim all-MiniLM-L6-v2; nil OK if backend embeds
}
type SearchResult struct { Entry Entry; Score float32 } // 0..1, higher = more similar
type Store interface {
    Put(ctx, Entry) error                                   // idempotent on SmashID, upsert
    Get(ctx, smashID) (Entry, bool, error)                  // (zero,false,nil) on not-found
    Search(ctx, query string, k int) ([]SearchResult, error)
}
```
Comments from source: writes are **best-effort** (resolver logs+continues on failure, don't block smash flow). claude-flow embeds server-side, so `Embedding` can be `nil`.

---

## 3. Memory debate — reconciled

**Three takes considered:**
- **User:** RAG for everything.
- **Gemini:** MD (static) + Vector RAG (resolutions/postmortems) + skip code-AST.
- **Mine:** Compression is session-length, not memory-format — RAG won't fix it. SQLite for exact-match dedup (load-bearing) sits *in front of* RAG; semantic similarity is wrong for "seen this thread before?" — two incidents can be 0.91 similar. MD for slow-changing playbooks. RAG for semantic recall on postmortems.
- **Scaffold (discovered mid-conv):** Already chose RAG via `Entry.Embedding` + `claude-flow` MCP. `Bug.ID` exact-match dedup in `cmd/scheduler` (in-memory; persistence punted). Routing config + playbooks not in scaffold.

**Reconciled:**
| Layer | Store | Status | Lives in |
|---|---|---|---|
| Bug dedup (exact) | In-memory map by `Bug.ID` | designed | `cmd/scheduler` |
| Op state (queue, history) | In-memory; SQLite optional | designed in-memory | `cmd/scheduler` |
| Routing config | YAML | **Phase 1** | new `config/`, `internal/config/` |
| Playbooks (priority, persona, conventions) | MD | **Phase 4** | new `playbooks/` |
| SMASHed memory (semantic recall) | claude-flow MCP via `Store` | iface ✓, **impl Phase 3 (top)** | `internal/memory/claudeflow.go` |
| Code search | live grep/read in resolver | designed | `cmd/resolver` |

**Constraint:** `Bug.ID` exact-match must sit in front of RAG. Scaffold already enforces this — don't merge layers.

**My biggest correction:** I'd argued "RAG only later, when corpus exists." Wrong here — team's existing `/smash` skill at `~/tau/.claude/commands/smash.md` already writes to claude-flow MCP, so nr-smash inherits a warm corpus from PR-1.

**On compression:** none of the stores fix it. Fix is **architectural** — scaffold's per-component split into separate `cmd/*` binaries enforces short stateless runs (each cmd run is minutes long, exits, next run fresh). Compression risk by-design ~zero.

**On memory-as-replacement-for-work (locked this session):** memory does NOT auto-capture an agent run's reasoning, tried-and-failed steps, or transcript. It captures only what `Put()` writes — `Entry.FixShape` is the entire payload of cross-run knowledge. So `FixShape` must be written like a commit message for future-Claude: concrete, specific, includes the *why*. Vague FixShapes degrade the corpus.

**Short-circuit decision (locked this session):** if `Search()` returns a hit with `score >= short_circuit_threshold` (config), resolver SKIPS the smash entirely. Output is the past resolution; no new clone/fix/test. No hypothesis-tier middle ground; no human gate. Better-safe-than-sorry calibration handles false-positive risk. See §7.6, §13.1.

---

## 4. This session log

1. User proposed RAG-first memory; I decomposed into ID-dedup vs semantic vs config vs routing.
2. User raised compression concern; I reframed (session-length, not format).
3. User asked about cross-repo bugs → subagent-per-repo + coordinator + YAML+SQLite config.
4. User shared Gemini's design (MD + RAG + skip AST); compared, agreed on MD/RAG, disagreed on missing SQLite dedup.
5. User invoked `/nr:handoff`; I generated briefing **without reading repo first** — inaccurate (claimed lang unknown; it's Go).
6. **Corrective re-read:** read `ARCHITECTURE.md`, `internal/memory/memory.go`, all 5 `docs/components/*.md`, `README.md`, `go.mod`. Discovered scaffold already commits to RAG. Built 6-phase plan; created 6 tasks.
7. User: "codebase already has memories?" → explained interface-vs-impl distinction.
8. User: "what for better/efficient memory?" → reframed phase order (3 first).
9. User asked if claude-flow MCP installed. It wasn't. Verified `claude-flow@alpha` resolves on npm; confirmed `claude-flow mcp start`; verified server-side ONNX embedding (`all-MiniLM-L6-v2`, 384-dim) matches scaffold. Installed project-scope: `claude mcp add claude-flow --scope project -- npx -y claude-flow@alpha mcp start`. `.mcp.json` written. `claude mcp list` → `✓ Connected`. Tools not visible in current session — need restart.
10. User asked for detailed handoff before restart → expanded into full doc.
11. (prior turn) User asked to compact handoff for token efficiency → rewrote.
12. (this turn — post-restart, MCP visible) User clarified memory mental model: "I just want to store the final resolution so a repeat issue can be answered directly without re-running the smash." → reframed as short-circuit pattern. User initially agreed to hybrid (short-circuit ≥0.90 + hypothesis 0.65–0.90), then changed mind: **pure short-circuit, very high threshold, no human gate, no hypothesis tier**. Confirmed: threshold `0.95` placeholder, fires resolver-side, output `.smash/<id>/short-circuit.md` + `Status=resolved-via-memory`. Probed live MCP: stored DI-1234 fixture, retrieved cleanly, searched near-paraphrase query → **score 0.6366** (vs 0.95 threshold = would never fire). Discovered tool surface diverges from handoff §7.2 guesses: `memory_retrieve` (not `memory_get`), `similarity` (not `score`), `results` (not `matches`), embedding **768-dim mpnet** (not 384-dim MiniLM), search returns **truncated `value`** requiring follow-up retrieve. User opted to defer calibration to teammate; asked for full handoff update so a fresh teammate Claude Code session can pick up cold.

---

## 5. Phase plan (execution order)

| # | Phase | Goal | Status | LOC |
|---|---|---|---|---|
| 1 | **3** | `memory.Store` impl: claude-flow MCP | in_progress (MCP installed; coding pending) | ~120 + ~50 test |
| 2 | **4** | Static playbooks `playbooks/*.md` | outlined | ~3 md |
| 3 | **1** | Routing config `config/teams.yaml` + loader | detailed | ~150 + tests |
| 4 | **2** | Op state SQLite (optional) | outlined | ~200 |
| 5 | **5** | Wiring: scheduler reads config & fans out; resolver loads playbooks + memory | outlined | distributed |
| 6 | **6** | Tests + smoke run on one bug e2e | outlined | varies |

Phase numbers stable (TaskCreate IDs match); execution order differs.

---

## 6. Next-session first actions (teammate cold-start)

### 6.1 Verify MCP available

After cloning the repo, `.mcp.json` carries the claude-flow server config (project-scope). Claude Code will prompt to approve project-scope MCP servers on first launch in this directory — say yes.

```bash
claude mcp list 2>&1 | grep claude-flow
# Expected: claude-flow: npx -y claude-flow@alpha mcp start - ✓ Connected
```

If `! Needs authentication` or `✗ Failed`: halt and ask. There is no auth on claude-flow itself — failure usually means npm couldn't resolve the package (network), or the `-y` flag was missing from `npx -y` (rerun fixes it; `-y` is mandatory or npx prompts and the MCP server blocks).

First MCP startup downloads ~30MB ONNX embedding model from HuggingFace and can take 30s+. Don't panic.

### 6.2 Confirm tools visible

Search system prompt for `mcp__claude-flow__memory_`. Expect: `memory_store`, `memory_retrieve`, `memory_search`, `memory_list`, `memory_delete`, `memory_stats`. (Plus many orchestration / hive-mind / agent tools — irrelevant to nr-smash.)

If missing: restart Claude Code, or `/mcp` refresh.

### 6.3 Probe results — captured live, do not re-probe

The live probe (Put + Retrieve + Search) was completed in the prior session. Findings are baked into §7.2 (real tool shapes), §13.1 (threshold evidence), and §16 (gotchas). **You should not need to re-run the probe.**

If you do want a quick sanity check that tools work end-to-end in your session before coding:

```
mcp__claude-flow__memory_store(namespace="<your-name>-probe", key="hello", value='{"test":1}')
mcp__claude-flow__memory_retrieve(namespace="<your-name>-probe", key="hello")
```

Should return `{success:true, ...}` and `{found:true, value:{test:1}, ...}` respectively.

The prior probe's fixture lives at `namespace=nr-smash-probe, key=probe-001`. Harmless; can be deleted with `mcp__claude-flow__memory_delete` or left alone.

### 6.4 Run the threshold calibration battery (§13.1) BEFORE locking the threshold value

5 min, ~10 MCP calls. Result is a defensible `defaults.short_circuit_threshold` value to write into `config/teams.example.yaml` during Phase 1.

### 6.5 Then implement Phase 3 (§7), then Phase 4 (§8). Phase 4 is pure markdown.

---

## 7. Phase 3 — full impl plan

**Status: design locked, MCP tool shapes probed live, implementation pending.**

### 7.1 Files to create

```
internal/memory/
├── memory.go                  # exists — DO NOT modify (contract change needs [contract] PR)
├── claudeflow.go              # NEW — ClaudeFlowStore impl
├── claudeflow_test.go         # NEW — fixture-driven smoke test
├── mcp.go                     # NEW — minimal MCPCaller transport adapter (os/exec)
└── testdata/fixtures.json     # NEW — 5 known Entry records (§7.5)
```

### 7.2 Real MCP tool surface (probed live this session — do not re-guess)

#### `memory_store(key, value, namespace?, tags?, ttl?, upsert?)`

**Request:**
- `key` (string, required) — opaque ID; use `Entry.SmashID`.
- `value` (string|object, required) — JSON-encoded Entry. Pass as **string** for portability.
- `namespace` (string, default `"default"`) — use `"nr-smash"` in prod, `"nr-smash-test-<ts>"` for tests.
- `tags` ([]string, optional) — searchable. Recommended: `[Repo, Pattern, Tier]`.
- `ttl` (number sec, optional) — leave unset; entries are forever.
- `upsert` (bool, default `false`) — set `true` so re-Put on same SmashID overwrites.

**Response (verbatim shape from probe):**
```json
{
  "success": true,
  "key": "probe-001",
  "namespace": "nr-smash-probe",
  "stored": true,
  "storedAt": "2026-06-12T13:01:18.526Z",
  "hasEmbedding": true,
  "embeddingDimensions": 768,
  "backend": "sql.js + HNSW",
  "storeTime": "66.50ms"
}
```

#### `memory_retrieve(key, namespace?)`

(Note: tool name is `memory_retrieve`, **not** `memory_get` as old handoff guessed.)

**Request:** just `key` and optional `namespace`.

**Response on hit (verbatim shape from probe):**
```json
{
  "key": "probe-001",
  "namespace": "nr-smash-probe",
  "value": { "smash_id": "DI-1234", "...": "...full Entry as parsed object..." },
  "tags": ["nr-smash", "probe", "mssql", "context-timeout"],
  "storedAt": 1781269278461,
  "updatedAt": 1781269278461,
  "accessCount": 1,
  "hasEmbedding": true,
  "found": true,
  "backend": "sql.js + HNSW"
}
```

Notice `value` comes back as a **parsed object** (map), not a JSON string. Handle both in case of server changes.

**Response on miss:** `found: false` (or `value` absent — handle either).

#### `memory_search(query, namespace?, limit?, threshold?, smart?)`

**Request:**
- `query` (string, required).
- `namespace` (string, default `"default"`).
- `limit` (number, default 10).
- `threshold` (number 0–1, default 0.3) — server-side pre-filter; can leave at default. Note: this is a *minimum-similarity gate at the API level*, separate from our `short_circuit_threshold`. Don't conflate them.
- `smart` (bool, default false) — query expansion + MMR diversity. **DO NOT enable for short-circuit lookups** — duplicate detection wants raw similarity, not diversified results.

**Response (verbatim shape from probe):**
```json
{
  "query": "mssql connection hang missing context timeout",
  "results": [
    {
      "key": "probe-001",
      "namespace": "nr-smash-probe",
      "value": "{\"smash_id\":\"DI-1234\",\"bug_id\":\"jira:DI-1234\",\"repo\":\"newrel...",
      "similarity": 0.6366047263145447
    }
  ],
  "total": 1,
  "searchTime": "5.07ms",
  "backend": "HNSW + sql.js"
}
```

**🚨 GOTCHA: `value` in search results is TRUNCATED.** The string is cut off mid-content (~60 chars in the probe). To get the full Entry after a search match, follow up with `memory_retrieve(key)`. Adds one round-trip per match. Don't try to JSON-parse the truncated string.

#### Renamed fields vs old handoff guesses
| Old guess | Real |
|---|---|
| `memory_get` (tool) | `memory_retrieve` |
| `match.score` | `result.similarity` |
| `matches` (array) | `results` |
| 384-dim `all-MiniLM-L6-v2` | **768-dim mpnet-class** |

### 7.3 `claudeflow.go` — design (real shapes)

```go
package memory

import (
    "context"
    "encoding/json"
    "fmt"
)

type MCPCaller interface {
    Call(ctx context.Context, tool string, args map[string]any) (map[string]any, error)
}

type ClaudeFlowStore struct {
    namespace string
    mcp       MCPCaller
}

func NewClaudeFlowStore(mcp MCPCaller, namespace string) *ClaudeFlowStore {
    if namespace == "" {
        namespace = "nr-smash"
    }
    return &ClaudeFlowStore{namespace: namespace, mcp: mcp}
}

func (s *ClaudeFlowStore) Put(ctx context.Context, e Entry) error {
    body, err := json.Marshal(e)
    if err != nil {
        return fmt.Errorf("marshal entry: %w", err)
    }
    _, err = s.mcp.Call(ctx, "memory_store", map[string]any{
        "namespace": s.namespace,
        "key":       e.SmashID,
        "value":     string(body),
        "tags":      []string{e.Repo, e.Pattern, e.Tier},
        "upsert":    true,
    })
    if err != nil {
        return fmt.Errorf("memory_store: %w", err)
    }
    return nil
}

func (s *ClaudeFlowStore) Get(ctx context.Context, smashID string) (Entry, bool, error) {
    res, err := s.mcp.Call(ctx, "memory_retrieve", map[string]any{
        "namespace": s.namespace,
        "key":       smashID,
    })
    if err != nil {
        return Entry{}, false, fmt.Errorf("memory_retrieve: %w", err)
    }
    if found, _ := res["found"].(bool); !found {
        return Entry{}, false, nil
    }
    // value can come back parsed-object OR string — handle both.
    var e Entry
    switch v := res["value"].(type) {
    case map[string]any:
        b, _ := json.Marshal(v)
        if err := json.Unmarshal(b, &e); err != nil {
            return Entry{}, false, err
        }
    case string:
        if err := json.Unmarshal([]byte(v), &e); err != nil {
            return Entry{}, false, err
        }
    default:
        return Entry{}, false, fmt.Errorf("unexpected value type %T", v)
    }
    return e, true, nil
}

func (s *ClaudeFlowStore) Search(ctx context.Context, query string, k int) ([]SearchResult, error) {
    res, err := s.mcp.Call(ctx, "memory_search", map[string]any{
        "namespace": s.namespace,
        "query":     query,
        "limit":     k,
        // do NOT pass smart:true — short-circuit needs raw similarity, not diversified results
    })
    if err != nil {
        return nil, fmt.Errorf("memory_search: %w", err)
    }
    raw, _ := res["results"].([]any)
    out := make([]SearchResult, 0, len(raw))
    for _, r := range raw {
        m, ok := r.(map[string]any)
        if !ok {
            continue
        }
        key, _ := m["key"].(string)
        sim, _ := m["similarity"].(float64)
        // GOTCHA: value here is TRUNCATED. Re-fetch full Entry via Get.
        full, found, err := s.Get(ctx, key)
        if err != nil || !found {
            continue // skip malformed; never fail whole search
        }
        out = append(out, SearchResult{Entry: full, Score: float32(sim)})
    }
    return out, nil
}

var _ Store = (*ClaudeFlowStore)(nil)
```

### 7.4 `mcp.go` — transport options

**Option A — `os/exec` calling `npx claude-flow@alpha mcp exec` (hackathon-fast):**

```go
type CLIMCPCaller struct { binary string; args []string } // npx, ["-y","claude-flow@alpha","mcp","exec"]
// Call: json.Marshal(args) on stdin; exec npx ... <tool>; capture stdout; unmarshal map.
// Wrap exec.ExitError to surface stderr.
```

**Caveat:** confirm `claude-flow mcp exec` actually exists with `npx claude-flow@alpha mcp --help`. If the subcommand is named differently (e.g. `mcp call`, or there is no CLI exec at all), fall through to Option B.

**Option B — proper Go MCP client (`github.com/mark3labs/mcp-go`, recommended if A doesn't exist):**

Spin up the MCP server once at process start (`npx -y claude-flow@alpha mcp start`), connect over stdio, and call tools through the client library. Slightly more code, but a real MCP transport instead of CLI-shelling.

Pick Option A first; pivot to B if `mcp exec` isn't a real subcommand.

### 7.5 `testdata/fixtures.json` — 5 known entries

Verbatim list (don't paraphrase `fix_shape` — tests + calibration battery assert text):

1. `DI-1234` / `jira:DI-1234` / `newrelic/nri-mssql` / `missing-context-timeout` / "Added context.WithTimeout(15s) to all sql.QueryContext calls in collector.go and inventory.go." / [12,3] / standard / PR#142 / 2026-05-01T10:00:00Z
2. `DI-1240` / `jira:DI-1240` / `newrelic/nri-postgresql` / `swallowed-error` / "Replaced ignored err returns with logged + propagated errors in connection-pool init." / [8,4] / standard / PR#87 / 2026-05-15T09:30:00Z
3. `freetext-mssql-pool-leak` / `slack:C0AB12CD/p1717000000123456` / `newrelic/nri-mssql` / `goroutine-leak` / "Added defer rows.Close() in iterateAndCollect; added context cancellation propagation in pool.go." / [6,0] / customer-facing / PR#151 / 2026-06-02T15:20:00Z
4. `DI-1255` / `jira:DI-1255` / `newrelic/nri-mssql` / `nil-deref-on-empty` / "Added len(rows)==0 guard before indexing rows[0] in metric extraction." / [3,0] / trivial / PR#154 / 2026-06-08T11:10:00Z
5. `DI-1260` / `jira:DI-1260` / `newrelic/nri-mssql` / `sql-no-timeout` / "Replaced db.Query with db.QueryContext + 30s timeout in long-running inventory queries." / [15,6] / standard / PR#158 / 2026-06-10T14:45:00Z

(All `passed_tests:true`, `pr_url:https://github.com/<repo>/pull/<num>`.)

### 7.6 Wiring (short-circuit only — locked design)

**`cmd/search/main.go`** (read-only spot-check CLI): `flag` for `-q`, `-k=5`, `-namespace=nr-smash`. `store := memory.NewClaudeFlowStore(memory.NewCLIMCPCaller(), *namespace)`. Print tab-separated `SmashID similarity(.3f) Pattern Repo PRURL`, then truncated FixShape to 100 chars.

**`cmd/resolver/main.go`** — short-circuit branch at top of work loop:

```go
const similaritySearchBudget = 5 * time.Second
// shortCircuitThreshold loaded from teams.yaml.defaults.short_circuit_threshold
// Phase 1 default: 0.95 (PLACEHOLDER — calibrate via §13.1 before locking)

func handleBug(ctx context.Context, bug bug.Bug, store memory.Store, threshold float32) error {
    sctx, cancel := context.WithTimeout(ctx, similaritySearchBudget)
    defer cancel()

    hits, err := store.Search(sctx, bug.Title+" "+bug.Body, 1)
    if err != nil {
        log.Printf("memory search failed (continuing cold): %v", err) // best-effort, ARCHITECTURE.md:148
    }

    if len(hits) > 0 && hits[0].Score >= threshold {
        return shortCircuit(bug, hits[0])
    }
    return coldSmash(ctx, bug, store)
}

func shortCircuit(b bug.Bug, hit memory.SearchResult) error {
    b.SmashID = generateSmashID(b)            // fresh SmashID for traceability of THIS bug instance
    b.Status = "resolved-via-memory"           // new enum value, see §15

    outDir := filepath.Join(".smash", b.SmashID)
    if err := os.MkdirAll(outDir, 0o755); err != nil {
        return err
    }
    md := fmt.Sprintf(`# Short-circuit resolution

Bug: %s — %s
Matched against: %s (similarity %.3f)

## Past resolution

%s

## Past PR

%s — shipped %s

## Action

This is a probable duplicate of a previously-fixed bug. Apply the same fix or close as duplicate.
`, b.ID, b.Title, hit.Entry.SmashID, hit.Score, hit.Entry.FixShape, hit.Entry.PRURL, hit.Entry.ShippedAt.Format(time.RFC3339))

    return os.WriteFile(filepath.Join(outDir, "short-circuit.md"), []byte(md), 0o644)
}
```

**Resolver does NOT write a new `memory.Entry` on short-circuit** — the existing entry is already the answer; writing a near-duplicate would pollute the corpus. Only the `coldSmash` branch writes new entries on success.

### 7.7 DoD

- [ ] `claudeflow.go` compiles, `var _ Store = (*ClaudeFlowStore)(nil)` passes
- [ ] `mcp.go` exists with `CLIMCPCaller` (Option A) or `mcp-go` adapter (Option B)
- [ ] `go test ./internal/memory/...` green
- [ ] `testdata/fixtures.json` exists, loaded by tests
- [ ] `cmd/search` builds, prints reasonable output for `-q "missing context timeout"`
- [ ] Resolver short-circuit branch integrates and compiles
- [ ] `Status=resolved-via-memory` added to `internal/bug/bug.go` enum (§15) — `[contract]` PR
- [ ] `memory.go` package doc points at `ClaudeFlowStore` as default; embedding-dim comment corrected to 768 — same `[contract]` PR
- [ ] `.mcp.json` committed (currently UNTRACKED — see §17)
- [ ] `NRSMASH_LIVE_MCP=1 go test` passes once against real claude-flow
- [ ] Calibration battery (§13.1) run, threshold value picked, written to `config/teams.example.yaml`
- [ ] Phase 3 task → completed via TaskUpdate

---

## 8. Phase 4 — playbooks

### 8.1 Files

```
playbooks/{prioritization,customer-comms,team-conventions}.md
internal/playbooks/playbooks.go     # Load(dir) → map[name]content
```

### 8.2 `playbooks/prioritization.md`

Source: `docs/components/triage-agent.md:24-32` (tier rules) + `ARCHITECTURE.md:90-98` (priority factors). Cover: Tier table (Architectural / Customer-facing / Trivial / Standard — see §11), ambiguous → leave Tier empty, scheduler decides. Priority factors: tier weight (customer-facing > architectural > standard > trivial), label boosts (sev1, customer-asked = constant boost), age (accumulates, capped), optional repo weight. Higher score = more urgent. Recomputed every `Pop()`.

### 8.3 `playbooks/customer-comms.md`

Applies when `Tier=customer-facing`. Resolver invokes customer-comms agent (Haiku, Rahul's voice) producing 3 drafts: 1-line / 1-paragraph / full-detail. All written to `<output-dir>/<smash-id>/customer-response-{1line,paragraph,full}.txt`, referenced from change-doc. Human picks at PR review. **Voice rules: copy verbatim from `~/tau/.claude/commands/smash.md` "Customer-facing sub-team protocol"** — leave TODO marker if not yet copied.

### 8.4 `playbooks/team-conventions.md`

- **PR title prefix:** `resolver:` / `triage:` / `collector:` / `scheduler:` / `search:` / `contract:` (for changes in `internal/{bug,memory,priority}`).
- **Branch naming:** `smash/<smash-id>`. SmashID = `DI-1234` (Jira) or `freetext-<slug>` (Slack) per `docs/components/resolver.md:39`.
- **AIDefence:** LOW → safe to PR; MEDIUM/HIGH → halt, `Status=needs-human-review`.
- **No `--no-verify`** — project-wide ban; if hook fails, fix the input.

### 8.5 `internal/playbooks/playbooks.go`

```go
func Load(dir string) (map[string]string, error)
// reads *.md from dir; key = basename without ".md"; value = file body.
// Used by resolver to inject playbooks into LLM system prompt.
```

### 8.6 DoD

- [ ] 3 `playbooks/*.md` reviewed for accuracy
- [ ] `internal/playbooks/playbooks.go` with `Load(dir)`
- [ ] resolver loads playbooks at startup → concatenates into system prompt
- [ ] Phase 4 task → completed

---

## 9. Phase 1 — routing config

### 9.1 Files

```
config/{teams.example.yaml, teams.yaml(gitignored)}
internal/config/{config.go, config_test.go}
internal/config/testdata/{valid.yaml, missing_team_name.yaml, multi_team.yaml}
```

### 9.2 `config/teams.example.yaml`

```yaml
version: 1
defaults:
  poll_interval: 5m
  similarity_threshold: 0.65
teams:
  - name: di
    slack_channels: ["C0AB12CD"]   # #help-db-o11y
    jira_projects: ["DI"]
    repos: [newrelic/nri-mssql, newrelic/nri-postgresql]
    primary_repo: newrelic/nri-mssql
    label_boosts: { sev1: 100, customer-asked: 50 }
```

### 9.3 `internal/config/config.go` — design

```go
type Root struct { Version int; Defaults Defaults; Teams []Team }
type Defaults struct { PollInterval time.Duration; SimilarityThreshold float32 }
type Team struct {
    Name string; SlackChannels []string; JiraProjects []string;
    Repos []string; PrimaryRepo string; LabelBoosts map[string]int
}
func Load(path string) (*Root, error)         // ReadFile → yaml.Unmarshal → validate
func (r *Root) Team(name string) (*Team, bool)
```

**`validate()`:** version==1; ≥1 team; team.Name required & unique; ≥1 of slack_channels/jira_projects/repos; if primary_repo set, must be in repos.

### 9.4 Wiring

`--config` flag on `cmd/{collector,triage,scheduler}`. Keep per-flag args one release as deprecated overrides w/ warning.

### 9.5 DoD

- [ ] `teams.example.yaml` committed; real `teams.yaml` gitignored
- [ ] `config.go` + tests pass
- [ ] 3 cmd binaries accept `--config`
- [ ] README quick-start updated
- [ ] Phase 1 task → completed

---

## 10. Phase 2 — SQLite ops state (optional)

### 10.1 Schema (`.smash/state.db`)

```sql
CREATE TABLE seen_bugs(
  bug_id TEXT PRIMARY KEY, first_seen_at TEXT, last_seen_at TEXT,
  status TEXT,           -- queued|resolving|resolved|aborted|needs-human-review
  smash_id TEXT, pr_url TEXT
);
CREATE INDEX idx_seen_bugs_status ON seen_bugs(status);
CREATE TABLE runs(run_id TEXT PRIMARY KEY, started_at TEXT, ended_at TEXT,
                  bugs_processed INTEGER DEFAULT 0, errors INTEGER DEFAULT 0, notes TEXT);
CREATE TABLE keyword_repo_affinity(keyword TEXT, repo TEXT, hits INTEGER DEFAULT 0,
                                    PRIMARY KEY(keyword,repo));
```

### 10.2 Package

`internal/state/state.go`: `Open`, `MarkSeen`, `MarkResolved`, `BumpAffinity`, `LookupAffinity`. Use `modernc.org/sqlite` (pure Go, no CGO).

### 10.3 Decision

Scaffold says skip; my recommendation: **do it**, ~150 LOC, saves demo embarrassment if scheduler restarts mid-demo. User to confirm before starting.

---

## 11. Phases 5+6

**Phase 5 (wiring, after 3+4):**
- `cmd/scheduler` loads `config/teams.yaml`, fans out per-team to collector + triage.
- `cmd/resolver` loads `playbooks/*.md` at startup, holds in memory, injects into LLM system prompt.
- `cmd/{collector,triage}` write through `internal/state` (if Phase 2) or in-memory dedup.
- `cmd/resolver` writes `memory.Entry` after every successful smash.
- `--dry-run` flag on resolver for demo.
- DoD: e2e: scheduler launches → collector polls → triage classifies → scheduler queues → resolver pops → smash flow → memory entry written.

**Phase 6:** golden-file tests per component (testdata/, no network). Live smoke on `nri-mssql` with `--dry-run`, verifying `.smash/<smash-id>/` artefacts: branch in local clone, `change-doc.md`, `proposal.md` (if architectural), `customer-response-*.txt` (if customer-facing), `memory.Entry` in claude-flow MCP.

---

## 12. Useful commands

```bash
claude mcp list                                    # verify MCP
go build ./... && go test ./...                    # build/test all
go test ./internal/memory/...                      # memory tests only
NRSMASH_LIVE_MCP=1 go test ./internal/memory/... -run Live -v  # live MCP
gofmt -l -s . && go vet ./...                      # format + vet
go get gopkg.in/yaml.v3                            # Phase 1
go get modernc.org/sqlite                          # Phase 2
npx claude-flow@alpha mcp --help                   # discover exec syntax
npx claude-flow@alpha mcp exec --help
```

---

## 13. Open questions

1. **[OPEN — RUN THIS FIRST] Threshold calibration** — placeholder `0.95` confirmed too high by probe (near-paraphrase scored 0.6366). See §13.1 for calibration battery. Result lands in `config/teams.example.yaml.defaults.short_circuit_threshold`.
2. **[RESOLVED]** ~~MCP tool exact shape~~ — probed live; real shapes baked into §7.2.
3. **Go MCP transport** — Option A (`os/exec` + `npx claude-flow@alpha mcp exec`) vs Option B (`github.com/mark3labs/mcp-go` real client). See §7.4. Verify subcommand exists with `npx claude-flow@alpha mcp --help` before committing to A.
4. **Commit `.mcp.json` and HANDOFF.md** — both currently untracked on `main`. Recommend yes so teammate inherits. User to do via `git add .mcp.json HANDOFF.md && git commit`.
5. **Phase 2 (SQLite ops state)?** — scaffold says skip; handoff recommends do it (~150 LOC, demo-restart safety). User decides before Phase 5.
6. **Playbook source-of-truth** — copy snapshot from `~/tau/.claude/commands/smash.md` vs read by reference. Recommend snapshot.
7. **`alpha` tag pinning** — `claude-flow@alpha` resolves to `3.10.42` today. Pin specific version in `.mcp.json` if reliability matters for demo.
8. **Embedding model details** — first `claude-flow mcp start` downloads ~30MB ONNX from HuggingFace. Pre-warm demo machine if no internet at demo. Probed model is **mpnet-class, 768-dim** (not MiniLM 384-dim as `memory.go` comment claims). Update `memory.go` comment in the `[contract]` PR that adds `Status=resolved-via-memory`.
9. **Namespace strategy** — `nr-smash` separates from `/smash` skill writes; switch to shared if team wants warm corpus directly. Low priority.
10. **Memory write best-effort policy** — resolver continues on memory write failure (`ARCHITECTURE.md:148`). Make sure logs make it visible — silent best-effort is hard to debug.
11. **Search value truncation** — `memory_search` returns truncated `value` strings; `claudeflow.go` design re-fetches via `memory_retrieve(key)` per hit. Acceptable for k≤5; revisit if scale grows or if `memory_search` gains a `full_value:true` flag in future claude-flow versions.

### 13.1 Threshold calibration battery (DO THIS BEFORE LOCKING THRESHOLD)

The probe in this session showed:
- Stored: `"Added context.WithTimeout(15s) to all sql.QueryContext calls in collector.go and inventory.go."` (DI-1234, mssql, missing-context-timeout)
- Query: `"mssql connection hang missing context timeout"` (near-paraphrase)
- Result: **similarity 0.6366**

That is the *high-similarity floor* for this embedding model on this corpus. Real-world repeat bugs described in different words will score lower. So:
- Threshold `0.95` → never fires (false-negative-everything)
- Threshold `0.80` → maybe over-fires (false positives)
- Need empirical evidence to pick well

**Battery procedure (~5 min, ~10 MCP calls):**

1. Store all 5 fixtures from §7.5 to `namespace=nr-smash-calibration` via `mcp__claude-flow__memory_store`.
2. Run these 5 queries via `mcp__claude-flow__memory_search` (limit=5, no `smart`):

| # | Query type | Example query | Expected band |
|---|---|---|---|
| 1 | Verbatim | `"Added context.WithTimeout(15s) to sql.QueryContext"` | ≥ 0.85 |
| 2 | Near-paraphrase | `"mssql connection hang missing context timeout"` | 0.60–0.70 |
| 3 | Same pattern, different repo | `"postgresql query hang no timeout"` | 0.50–0.60 |
| 4 | Different pattern, same repo | `"mssql nil dereference on empty result"` | 0.30–0.50 |
| 5 | Unrelated | `"yaml parse error in config loader"` | < 0.30 |

3. Record the top-1 similarity for each query.
4. **Pick threshold in the gap between query #2 (`near-paraphrase`) and query #3 (`same-pattern-different-repo`).** Skew toward the high end of that gap (better-safe-than-sorry — false-positive short-circuits send wrong fix to a real bug).
5. Cross-check: re-run query #2 with the chosen threshold mentally applied. It should pass (this is the kind of repeat bug we *want* to short-circuit). Re-run query #3. It should fail (different repo, different fix likely needed).
6. Write the value to `config/teams.example.yaml` as `defaults.short_circuit_threshold: <value>`.
7. Clean up: `mcp__claude-flow__memory_delete` for each calibration entry, or just leave the namespace (will be ignored by prod queries to `namespace=nr-smash`).

If the gap between #2 and #3 is small (< 0.05), the model is not discriminating enough for short-circuit at any threshold — escalate; we may need to add hypothesis-tier fallback after all, OR rely on `tags` filtering (e.g. require `tag=<repo>` match on top of similarity).

---

## 14. Tasks (TaskCreate IDs)

| ID | Phase | Status | Active form |
|---|---|---|---|
| 1 | Phase 1 (routing) | pending | Adding routing config layer |
| 2 | Phase 2 (SQLite) | pending | Adding SQLite state store |
| 3 | Phase 3 (memory.Store impl) | **in_progress** (MCP installed; coding pending) | Implementing memory store backend |
| 4 | Phase 4 (playbooks) | pending | Writing behavioral playbooks |
| 5 | Phase 5 (wiring) | pending | Wiring memory into components |
| 6 | Phase 6 (tests + smoke) | pending | Adding tests and smoke run |

---

## 15. Glossary + canonical enums

**Glossary:**
- **Bug** — `internal/bug/bug.Bug`. Cross-component record. Fields: `ID, Source, Title, Body, Repo, Tier, Priority, Status, SmashID`.
- **Smash** — one full resolver pass on a single bug (clone → fix → test → PR).
- **SmashID** — stable ID matching resolver branch (`smash/<smash-id>`) and change-doc (`.smash/<smash-id>.md`). Format: `DI-1234` (Jira) | `freetext-<slug>` (Slack).
- **Tier** — `trivial | standard | architectural | customer-facing`. Drives full-flow vs proposal-only vs invoke customer-comms agent.
- **Pattern** — categorical bug label, drives memory search hits. Enum below.
- **FixShape** — 1-paragraph fix description, written by resolver after success, used as starting hypothesis on similar bugs.
- **SMASHed memory** — resolver-write/search-read store, backed by claude-flow MCP via `memory.Store`.
- **claude-flow MCP** — external MCP server: `memory_store`/`memory_search` tools, HNSW vector index, `all-MiniLM-L6-v2` 384-dim embeddings.
- **AIDefence** — existing NR tool; scans diffs for risk pre-push. Resolver calls it; if risk > LOW, halt → `Status=needs-human-review`.

**Pattern enum** (`docs/components/resolver.md:92-103`):
`missing-context-timeout, swallowed-error, unbounded-retry, stale-cache, nil-deref-on-empty, goroutine-leak, sql-no-timeout, yaml-parse-error, field-required-missing, auth-failed`. Coin new ones in `kebab-case` and document in same PR. Sloppy labels degrade search.

**Tier triggers** (`docs/components/triage-agent.md`):
| Tier | Triggers |
|---|---|
| Architectural | Public API change, schema migration, breaks backward compat, spans 3+ packages, body contains "redesign"/"migration"/"breaking" |
| Customer-facing | Label `customer-asked`, severity `sev1`, body mentions specific customer name/account |
| Trivial | Single-file lint, typo, dep bump, deprecated API swap, LOC < 20 |
| Standard | Everything else |

**Source enum:** `Bug.Source ∈ {jira, slack, repo-issue, repo-pr-comment, repo-lint}`.
**Status enum:** `Bug.Status ∈ {queued, resolving, resolved, resolved-via-memory, aborted, needs-human-review}`. The `resolved-via-memory` value is set by the resolver short-circuit branch (§7.6); requires `[contract]`-prefixed PR adding it to `internal/bug/bug.go`.
**Bug.ID format:** Jira `jira:DI-1234`; Slack `slack:<channel>/p<ts>`; Repo issue `repo:<owner>/<repo>/issues/<num>`.

---

## 16. Gotchas

1. **Don't modify `internal/memory/memory.go`** — shared contract; needs `[contract]`-prefixed PR. Adding new file (`claudeflow.go`) in same package is fine.
2. **`claude-flow@alpha` tag drift** — `3.10.42` today, could move. Pin specific version in `.mcp.json` if demo reliability matters.
3. **First `claude-flow mcp start` is slow** — 23MB ONNX download, can take 30s+. Don't panic.
4. **Memory writes best-effort** (`ARCHITECTURE.md:148`) — resolver MUST continue on failure. Don't accidentally make it fatal.
5. **Mock subtlety:** `mockMCP.Call` returns same `storeReturns` every call. Extend if per-call differs.
6. **`npx -y` flag mandatory** — without `-y`, npx prompts on first download and MCP startup blocks. Always include.
7. **Embedding `nil` vs empty** — caller passes `Entry.Embedding=nil`; backend embeds. Don't pre-embed unless model verified (`all-MiniLM-L6-v2`, 384-dim).
8. **Score units** — claude-flow returns `[0,1]`. Resolver threshold 0.65. Don't accidentally use raw cosine distance (`1 - similarity`).
9. **Namespace pollution** — tests use `test-ns` or time-stamped namespace. Don't write tests to `nr-smash` namespace.
10. **`.mcp.json` portability** — assumes Node + npm in PATH. Document prereq in CONTRIBUTING.md for fresh-machine teammates.
11. **Hackathon docs at `~/tau/.claude/commands/smash.md`** — referenced by multiple `docs/components/*.md`. Update refs if it moves.
12. **No `--no-verify`** project-wide ban (CONTRIBUTING.md + future `playbooks/team-conventions.md`).
13. **`memory_search` returns truncated `value`** — the JSON string is cut off mid-content (~60 chars in the live probe). Don't try to parse it. Always follow up with `memory_retrieve(key)` to get the full Entry. Cost: ~10ms/match, k≤5 ⇒ acceptable.
14. **Tool name is `memory_retrieve`, not `memory_get`** — the older claude-flow docs (and the original handoff §6.3 / §7.2) had it wrong. Don't grep for `memory_get`.
15. **Search response field is `similarity`, not `score`. Array key is `results`, not `matches`.** Old guesses are wrong; live shapes captured in §7.2 are authoritative.
16. **Embedding is 768-dim mpnet-class, not 384-dim MiniLM.** `internal/memory/memory.go` comment is inaccurate. If anyone pre-embeds (currently no one does — caller passes `Embedding=nil`), match the actual model. Comment correction lands in the same `[contract]` PR as `Status=resolved-via-memory`.
17. **Probe scored 0.6366 for near-paraphrase on this model.** Don't pick a threshold north of ~0.85 without calibration evidence — `all-MiniLM-L6-v2`-style intuition is wrong here. See §13.1.
18. **`smart:true` on `memory_search`** — DO NOT enable for short-circuit lookups. It applies query expansion + MMR diversity, which is the *opposite* of what duplicate-detection wants.
19. **Resolver does NOT write a new `memory.Entry` on short-circuit** — pollutes the corpus with near-duplicates. Only the `coldSmash` branch writes new entries on successful PR. (See §7.6.)
20. **Server-side `threshold` in `memory_search` ≠ our `short_circuit_threshold`.** The server arg pre-filters at the API boundary (default 0.3); our config value is the duplicate-detection cliff applied client-side. Don't conflate.

---

## 17. Working tree state at handoff

- `.mcp.json` — created prior session, **untracked**, contains `claude-flow` server config. **Action required: commit before teammate clones**, otherwise teammate's clone won't have MCP config and they'll think MCP is broken.
- `HANDOFF.md` — this file, **untracked**, updated this turn with: locked short-circuit design, real MCP tool shapes (probed live), threshold calibration plan (§13.1), corrected gotchas, real return shapes for `memory_store` / `memory_retrieve` / `memory_search`. **Action required: commit before teammate clones.**
- One probe entry exists in claude-flow MCP at `namespace=nr-smash-probe`, `key=probe-001` (the DI-1234 fixture). Harmless — lives in `~/.claude-flow/...` server-local, doesn't follow the repo. Can clean up with `mcp__claude-flow__memory_delete(namespace="nr-smash-probe", key="probe-001")` or just leave it.
- All other repo files clean per `git status` at session start; no agent edits to scaffold files.

**Commit before pushing for teammate:**
```bash
git add .mcp.json HANDOFF.md
git commit -m "chore: claude-flow MCP project-scope + handoff with probe findings + locked short-circuit design"
git push
```

Teammate then `git clone`, opens Claude Code in the directory, accepts the project-scope MCP prompt, and starts at §6.

---

## 18. How to keep this fresh

I rewrite this each turn during memory-architecture work. Edit policy:
- §0 (TL;DR) only when immediate next step changes.
- §1 stable; only edit if scaffold structure changes.
- §4 (session log) append per turn.
- §6 (next-session actions) rewrite each turn to reflect new starting point.
- §7+ (phase plans) edit as we learn (real MCP tool shape replaces guessed shape).
- §13 (open questions) dump unresolved; when closed, move to §3.
- §15 (glossary) edit when terms evolve.
- §16 (gotchas) append-only.

Cold-start path: §0 + §6 → producing code in 5 min. Below is reference.
