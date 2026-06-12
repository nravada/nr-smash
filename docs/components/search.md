# Component — Search Skill

**Path:** `cmd/search`
**Owner:** _(set in `.github/CODEOWNERS`)_

## Job

Provide fast similarity search over the SMASHed memory store. Used by the resolver before every smash to find prior fixes that match the current bug's pattern; provided as a CLI for human spot-checks.

## Inputs

- A query string via `--q` (typically `bug.Title + " " + bug.Body`)
- A result count via `--k` (default 5)
- The configured memory store backend (env var `SMASH_MEMORY_BACKEND` — see ARCHITECTURE.md for options)

## Outputs

Top-K `memory.SearchResult` records, ordered by similarity score (highest first), with each result printed as a one-line summary:

```
<smash-id>  <score>  <pattern>  <repo>  <pr-url>
  fix shape: <one-paragraph FixShape, truncated to ~100 chars>
```

The resolver consumes the same data via the in-process `memory.Store.Search` interface — the CLI is for humans.

## Memory backend choices

See `ARCHITECTURE.md` for the discussion. Recommended for hackathon: **claude-flow MCP** (`memory_store` / `memory_search`). Reasons:
- Already populated by the existing `/smash` skill — warm patterns from day 1
- HNSW-backed with native vector search
- No additional service to deploy

Fallbacks if claude-flow MCP is unavailable:
- SQLite + sqlite-vec extension (local file, easy to demo)
- BoltDB + in-memory cosine similarity (Go-native, no extension dependency)

The choice is encapsulated behind `memory.Store`; switching backends is a one-file change.

## Similarity threshold

The resolver uses a configured threshold (suggested default: `0.65`). Below that, the resolver treats the smash as cold. Above it, the resolver uses the matched `Entry.FixShape` as a starting hypothesis (not as a verbatim fix).

## Hackathon checklist

- [ ] Implement `memory.Store` against the chosen backend (recommended: claude-flow MCP)
- [ ] CLI: pretty-print results
- [ ] Tests: with a fixture of 5 known entries, query similar text → expected ordering
- [ ] Embedding: pick one model and stick with it for the demo (suggested: `all-MiniLM-L6-v2`, 384-dim — what claude-flow uses by default)

## Out of scope (hackathon)

- Re-ranking with a cross-encoder
- Hybrid keyword + semantic search (semantic-only is fine)
- Cross-language embedding models (English-only)

## Related

- Shared types: `internal/memory`
- Architecture: `ARCHITECTURE.md`
