# Fix Plan: fix-023 — Traffic rate wrong / hangs / spikes

**Related:** changelog `ui-rate-ping-format` (units only; sampling left broken)
**Severity:** Medium (user-visible wrong rates; hang; multi-Gbps spikes)
**Reported by:** User (speedtest mismatch) + code analysis
**Date:** 2026-07-24

## Bug Summary

While connected, download/upload tiles are wrong:

1. Speedtest ~100 Mbps → UI ~700 Kbps (under-report / stale)
2. Rate tiles hang / stop updating for long stretches
3. Occasional absurd spikes (e.g. 1070 Mbps, multi-Gbps)

`formatRate` unit math (`bytes/s * 8` → Kbps/Mbps) is **correct**. Do not remove `* 8`.

## Root Cause Analysis (ranked)

| # | Cause | Symptom link | Where |
| - | ----- | ------------ | ----- |
| 1 | **First-sample spike:** `havePrev` flips true on the same tick that still has `prevRx/prevTx == 0`, so lifetime cumulative octets are treated as a 1s delta | Multi-Gbps spike on connect / first non-zero sample | `startTraffic` ~557–571 |
| 2 | **No elapsed-time normalization:** delta is passed to `formatRate` as if interval were exactly 1s. Skipped ticks (see #3) make multi-second growth look like 1s rate | ~1070 Mbps when real ~100 Mbps over ~10s | `startTraffic` delta → `formatRate` |
| 3 | **`tickBusy` held across slow `ActiveConnections`:** CAS skip while busy; ticker fires are dropped; when a tick finally finishes, huge byte delta + #2 → spike; during busy stretch UI freezes | Hang / stop updating + delayed spike | `startTraffic` ~550–609 |
| 4 | **`uint64` wrap check is dead:** `dRx := float64(rx - prevRx)` then `if dRx < 0` never true (unsigned wrap → huge positive float) | Rare absurd spike on counter reset / interface rebind | `startTraffic` ~561–567 |

`TrafficCounters` (GetIfEntry2 InOctets/OutOctets, soft-fail `(0,0,nil)`) is fine. Do not change VPN/win traffic APIs for this fix.

## Fix Strategy

### Option A: Minimal sampling fix (recommended)

Touch only `startTraffic` sampling loop (+ tiny `formatRate` test). Keep `formatRate` signature and body.

1. **Baseline-only first sample** — when establishing baseline (`!havePrev` or after soft-fail zeros), store `prevRx/prevTx` + timestamp, set `havePrev`, **do not** compute a rate that tick (leave `0 Kbps` or last shown).
2. **Elapsed normalization** — keep `prevAt time.Time`; each successful sample: `elapsed := now.Sub(prevAt).Seconds()`; if `elapsed <= 0` skip rate; else `bytesPerSec = deltaBytes / elapsed`; then `formatRate(bytesPerSec)`.
3. **Release `tickBusy` before slow work** — after counters + rate UI schedule, `tickBusy.Store(false)` **before** `ActiveConnections` (or run ActiveConnections outside the busy section). Rate path must not wait on connection enumeration.
4. **Safe uint64 delta** — only subtract when `rx >= prevRx` (else treat as reset: re-baseline, rate 0). Same for tx. Drop the useless `float64 < 0` checks.

- Risk: Low — display-only; no connect/disconnect path change
- Effort: S

### Option B: Thorough (not this ticket)

Rewrite traffic ticker architecture, debounce ActiveConnections on its own timer, EMA smoothing, etc.

**Recommended:** Option A only.

## Implementation Tasks

| Task | Agent | Files | Description |
| ---- | ----- | ----- | ----------- |
| 1 | Frontend Developer | `internal/ui/main_window.go` | Fix `startTraffic` per Option A: baseline-only first sample; `prevAt` + divide by elapsed; safe uint64 delta/reset; clear `tickBusy` before `ActiveConnections` (defer-safe: always clear on every exit path of the tick). Keep `formatRate` as-is. |
| 2 | Frontend Developer | `internal/ui/format_rate_test.go` (NEW) | Small table test for `formatRate`: 0 → `0 Kbps`; 125e3 bytes/s → `1.0 Mbps` (1e6 bits); 100 bytes/s → `1 Kbps` (800 bits rounded); boundary just below/above 1e6 bits. No Fyne, no ticker integration test. |
| 3 | Debugger/Reviewer | — | Verify acceptance criteria; `go test ./internal/ui/ -count=1`; `go build ./...`; `go vet ./internal/ui/...`. Manual: connect, run speedtest, watch tiles. |

**Sub-Agent Guidance:** Tasks 1–2 same agent, sequential (test after sampling edit). Task 3 after both.

## Acceptance Criteria

- [ ] First non-zero counter sample does **not** spike to multi-Gbps (baseline only).
- [ ] Sustained ~100 Mbps transfer shows UI on the order of tens–low-hundreds Mbps (not ~700 Kbps stuck, not ~1070 Mbps from multi-second deltas treated as 1s).
- [ ] Rate tiles keep updating ~1/s even when `ActiveConnections` is slow (no multi-second freeze solely from tickBusy+ActiveConnections).
- [ ] Counter reset / soft-fail `(0,0)` re-baselines without huge positive wrap rates.
- [ ] `formatRate` still uses bits (`* 8`); labels remain `Kbps` / `Mbps`.
- [ ] `go test ./internal/ui/ -count=1`, `go build ./...`, `go vet ./internal/ui/...` pass.

## What to Skip (YAGNI)

- Do **not** change `formatRate` math or remove `* 8`
- Do **not** redesign hero tiles / UI layout
- Do **not** rewrite `TrafficCounters` / GetIfEntry2 / PowerShell
- Do **not** move `ActiveConnections` to a new package or add deps
- Do **not** add EMA/smoothing, ring buffers, or config knobs
- Do **not** touch connect/disconnect, routes, ping, theme
- Do **not** call DevOps / change `build.ps1`

## Regression Risk

| Risk | Impact | Likelihood | Mitigation |
| ---- | ------ | ---------- | ---------- |
| Divide-by-near-zero elapsed | High rates | Low | Guard `elapsed <= 0` (or `< 1ms`) |
| tickBusy not cleared on early return | Permanent hang | Med if careless | Single clear path / `defer` after successful CAS |
| ActiveConnections without busy races UI | Log spam / status flicker | Low | Existing `prevConns` + `fyne.Do` state checks stay |
| Soft-fail zeros look like reset every tick | Stuck 0 Kbps | Med if interface flaps | Re-baseline only when `err != nil` or both counters 0 after had traffic; implementer: prefer re-baseline on `rx < prevRx` / zeros after `havePrev`, not on every zero pair if still connected |

## Rollback Strategy

Revert `startTraffic` sampling changes and delete `format_rate_test.go`. No config/schema/API surface.

## Version History

| Version | Date | Summary |
| ------- | ---- | ------- |
| v0.1.0 | 2026-07-24 | Initial fix plan |
