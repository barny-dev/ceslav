# ceslav

A command-line CSV tool. Supports counting and sorting records, selecting/dropping columns, and selecting, dropping, reordering and reversing records.

## Build and test

Requires Go 1.22.7 or newer. Use Go 1.27.4 on recent macOS versions.

```sh
go build -o out/ceslav .
go test ./...
go test -race ./...
go vet ./...
```

Integration tests build a temporary executable themselves; no prior build or `out` directory is required. CI checks Go 1.22.7 on Linux and Windows, the latest stable Go release on Linux, and Go 1.27.4 on macOS. The current macOS runner cannot execute some binaries produced by the older Go linker.

## Commands

```sh
# Count data records; consume an input header and write a count header.
ceslav row-count -j -k -i input.csv

# Sort by city ascending, then amount descending.
ceslav sort -j -k -b '+s%city,-d%amount' -i input.csv -o sorted.csv

# Headerless input: sort the second column numerically, descending.
ceslav sort -b '-d#1' < input.csv
```

All commands support:

| Option | Meaning |
| --- | --- |
| `-i`, `--input` | CSV input file; `-` (default) reads standard input |
| `-o`, `--output` | CSV output file; `-` (default) writes standard output |
| `-j`, `--header` | Consume the first CSV record as a header |
| `-k`, `--output-header` | Write an output header |

`row-count` excludes the input header from its count. Its optional output header is `row count`, so `-k` works with or without `-j`.

`sort` requires at least one `-b` / `--sort-by` key. Keys are comma-separated, in priority order:

| Part | Values |
| --- | --- |
| Direction | `+` ascending, `-` descending |
| Type | `s` string, `d` exact numeric |
| Column | `#0` zero-based index, `%name` exact header name |

Name-based keys require `-j`. Duplicate matching header names are ambiguous; use an index instead. For sorting, `-k` requires `-j`; input headers are omitted unless `-k` is set.

String ordering is case-sensitive Go string order, without locale collation. Numeric values use `math/big.Rat` syntax, including decimal, exponent and fraction forms such as `1.5`, `1e3` and `3/2`. Empty or malformed numeric keys are errors. Sorting retains all records in memory; equal keys currently have no stability guarantee.

## Column selection and removal

```sh
# Select and reorder columns; repeat the flag for each selector.
ceslav select -j -k --column '%city' --column '#0' -i input.csv

# Remove columns; keep the remainder in their original order.
ceslav drop -j -k --column '%debug' --column '%temporary' -i input.csv

# A comma inside a name is literal, not a separator between selectors.
ceslav select -j -k --column '%unit,price' -i input.csv
```

`select` and `drop` require at least one `-c` / `--column`. Both accept zero-based `#index` and exact `%name` references; names require `-j`. Duplicate references, including an index/name pair referring to the same column, are rejected. Dropping every column is an error because zero-column records cannot be represented unambiguously in CSV. An out-of-range index is checked against the header or first data record; headerless empty input has no width to validate. Input headers follow the same column transformation and are emitted only with `-k`.

## Row selection and removal

```sh
ceslav rows --first 10 -j -k -i input.csv
ceslav rows --last 10 -j -k -i input.csv
ceslav rows --first 10 --drop -j -k -i input.csv
ceslav rows --last 10 --drop -j -k -i input.csv

# Select data rows 1, 3, 4 and 7, in that order.
ceslav rows '1,3-4,7' -j -k -i input.csv

# Reorder rows, including a descending inclusive range.
ceslav rows '7,4-3,1' -j -k -i input.csv

# Remove listed rows; reverse the remaining rows.
ceslav rows '1,3-4,7' --drop --reverse -j -k -i input.csv

# Reverse all data rows, or just the first 10.
ceslav rows --reverse -j -k -i input.csv
ceslav rows --first 10 --reverse -j -k -i input.csv
```

Choose one selector list, `--first N` or `--last N`. N must be nonnegative and fit the platform's integer range. Selector lists use **one-based data-row numbers** and inclusive ranges, separated by commas. Row numbers must be positive decimal integers fitting a signed 64-bit integer; whitespace around numbers is allowed. Headers are excluded from numbering and counts; `-k` requires `-j` for all transformation commands.

Selector order determines output order. Descending ranges such as `4-2` select rows 4, 3 and 2. Repeated or overlapping selectors repeat those rows in the output; `--drop` removes their union and retains the order of remaining rows. Unavailable row numbers are ignored, and ranges are clipped to available input. An empty selector, row 0, a negative number, an open-ended range or malformed syntax is an error.

`--reverse` reverses the result of selection or dropping; without a selector list or first/last option, it reverses all data rows. The header remains first. `--drop` requires an explicit selection. A selector list cannot be combined with `--first` or `--last`.

Keeping zero records emits no data and does not read data records; with `-j` it still reads the required header and writes it if `-k` is set. Dropping zero preserves all data. If N exceeds the number of records, keeping retains all available records and dropping removes all of them. Keeping a prefix stops after N records, and keeping a selector list stops at its highest requested row, without waiting for input EOF. These early stops also apply with `--reverse`; trailing records are not parsed or validated. Dropping rows and reversing all input read through EOF.

Without reversal, prefix operations use constant auxiliary row storage; suffix operations use a ring buffer holding at most the smaller of N and the observed record count, without allocating N slots up front. Keeping a suffix emits data at EOF; dropping a suffix streams the prefix with an N-record delay. Explicit selection buffers only matching observed rows and emits them at completion in selector order, while dropping explicit selectors streams the remainder. Ranges are stored compactly without expanding their declared size. Reversal buffers the resulting data, except that reversing a kept suffix reuses its bounded ring buffer. Reversing all data therefore needs memory proportional to the input size.

Input uses comma-separated CSV, including quoted fields and quoted newlines. Records must have consistent field counts. Date/time operations, filtering and calculated columns are planned but not yet implemented.

Errors go to standard error with a nonzero exit code. CSV parse and processing errors identify the record; column errors identify the selector. Output files open only when a command first emits output (or completes successfully with empty output). Counting and sorting validate their full input before emitting output. Streaming transforms can emit partial output before encountering a later input error. Identical input/output files, including hard-link and symlink aliases, are rejected. Output write/flush/close errors are reported; output may be partial after an I/O failure. Redirecting stdout with the shell can truncate a destination before ceslav starts, so use `-o` when file identity checks are needed.

Cancellation closes the active input stream, including standard input when selected, to interrupt a blocked read. Standard input stays open on ordinary completion and deliberate early stop.

Use `ceslav --help`, `ceslav sort --help` and `ceslav row-count --help` for options. Set `CESLAV_LOG` to a file path to enable diagnostic logging.

Implementation progress is tracked in [docs/implementation-plan.md](docs/implementation-plan.md).
