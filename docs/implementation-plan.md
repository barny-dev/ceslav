# Implementation goals

| Goal | Status |
| --- | --- |
| 1. Reliable CSV processing, validation and tests | Complete |
| 2. Column projection and row selection | Planned |
| 3. Typed values and temporal conversions | Planned |
| 4. Row expressions, derived columns and filtering | Planned |
| 5. Expression sorting and stable ties | Planned |

## 1. Reliable CSV processing, validation and tests

- [x] Return input, processing and output errors to the CLI.
- [x] Stop processing on errors and complete exactly once on EOF or deliberate early stop.
- [x] Interrupt blocked input reads on cancellation and preserve the cancellation cause.
- [x] Finish cancellation cleanup before returning.
- [x] Open output files with correct write access and permissions; report write, flush and close errors.
- [x] Delay output creation and reject input/output file aliases before truncation.
- [x] Share column-name/index resolution and validate missing, ambiguous and out-of-range references.
- [x] Require sort keys and validate numeric values, including singleton inputs.
- [x] Build integration-test executables automatically and cover processing regressions.
- [x] Cover cancellation, cleanup ownership, sorting and counting with focused unit tests alongside I/O tests.
- [x] Maintain build, vet and race-test CI coverage across Linux, macOS and Windows.
- [x] Document the implemented commands and their input/output semantics.

## 2. Column projection and row selection

- [ ] Select columns in an explicit order, covering selection and reordering.
- [ ] Drop columns while retaining the original order of the remainder.
- [ ] Apply column projection to headers consistently.
- [ ] Keep or drop the first N data records using constant auxiliary row storage.
- [ ] Keep or drop the last N data records using a buffer bounded by N.
- [ ] Define zero, oversized and invalid counts; exclude headers from row counts.

## 3. Typed values and temporal conversions

- [ ] Support string, number, Boolean, date, time-of-day and datetime values.
- [ ] Preserve raw CSV cells and convert only fields used by an operation.
- [ ] Provide explicit default/custom temporal parsing and formatting.
- [ ] Define timezone, comparison, precision and serialization behavior.
- [ ] Validate dates, leap days, fractional seconds and equivalent instants with different offsets.

## 4. Row expressions, derived columns and filtering

- [ ] Compile row expressions once and resolve column references through the shared resolver.
- [ ] Support basic numeric, string, logical and temporal functions.
- [ ] Add columns from expressions, with consistent headers and defined collision behavior.
- [ ] Filter records using Boolean expressions and short-circuit logic.
- [ ] Report conversion/type errors with record, column and expression context.

## 5. Expression sorting and stable ties

- [ ] Add ascending/descending expression keys, including dates, times and datetimes.
- [ ] Preserve key priority and the existing sort-key shorthand.
- [ ] Evaluate and cache typed keys once per record.
- [ ] Preserve input order for equal keys.
- [ ] Share conversion and comparison semantics with derived columns and filtering.
