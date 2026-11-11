# ceslav implementation progress

Based on the functionality assessment dated 11 November 2026, baseline commit `d663537346ee3a0de4fd69da263bb15157edf7a1`.

Status distinguishes implementation, verification and merge. A completed implementation on a branch does not imply it has been merged.

| Step | Scope | Status |
| --- | --- | --- |
| 1 | Repair the foundation and make validation repeatable | Implemented locally; CI/review pending |
| 2 | Column projection and first/last row operations | Not started |
| 3 | Typed values and date/time conversions | Not started |
| 4 | Shared expressions, derived columns and filtering | Not started |
| 5 | Typed expression keys and stable sorting | Not started |

## Step 1 checklist

- [x] Replace cancellation-driven success with a synchronous runner returning real errors.
- [x] Stop immediately after initialization, header, row and input errors.
- [x] Complete exactly once at EOF or a deliberate stop; skip completion after errors/cancellation.
- [x] Correct file write access and permissions; report write/flush/close failures.
- [x] Delay output opening; reject input/output aliases before truncating.
- [x] Extract shared name/index resolution; reject missing, ambiguous and out-of-range columns.
- [x] Require sort keys; validate numeric values before heap insertion, including singleton input.
- [x] Make integration tests build a temporary executable and avoid pipe deadlocks.
- [x] Add regression and lifecycle tests.
- [x] Refresh README and CLI help.
- [x] Add CI for Linux/macOS/Windows on Go 1.22.7 and Linux on stable Go.
- [ ] Verify the hosted CI jobs.
- [x] Complete local review and record validation results.
- [ ] Open the implementation pull request.
- [ ] Merge after review.

## Execution log

### 2026-11-11 — Step 1 implementation

Replaced the three-goroutine CSV pipeline with one record-processing loop. Output opens lazily after processor validation; file identities are compared before truncation. Sorting now uses a shared column resolver and validates every numeric key before insertion. The test harness builds its executable in a temporary directory.

Local checks passed on Go 1.22.7 / Linux: `go test ./...`, `go test -race -count=1 ./...`, `go vet ./...`, and `git diff --check`. Regression coverage includes all 10 pre-existing cases, arbitrary/ambiguous named columns, singleton invalid values/indices, malformed and multiline CSV, file permissions and output errors, same-file aliases, and destination preservation on invalid input. Unit tests cover graceful early stop, completion counts, cancellation causes, and combined flush/close failures. Hosted CI and pull request review are still pending.

## Remaining scope

Step 2 adds `select`, `drop` and `rows` (keep/drop first/last N), reusing the resolver and early-stop lifecycle.

Step 3 introduces explicit numeric, string, Boolean, date, time-of-day and datetime values, while keeping CSV cells as strings at the I/O boundary.

Step 4 shares a compiled scalar expression evaluator between `derive` and `filter`.

Step 5 reuses that evaluator for temporal/expression sort keys, caches keys per row, and makes equal keys stable. The existing heap and repeated comparison-time numeric parsing remain until that step.
