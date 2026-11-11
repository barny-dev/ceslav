# ceslav

A command-line CSV tool. Currently supports counting records and sorting by string or numeric columns.

## Build and test

Requires Go 1.22.7 or newer.

```sh
go build -o out/ceslav .
go test ./...
go test -race ./...
go vet ./...
```

Integration tests build a temporary executable themselves; no prior build or `out` directory is required. CI checks Linux, macOS and Windows with the minimum Go version, and Linux with the latest stable Go release.

## Commands

```sh
# Count data records; consume an input header and write a count header.
ceslav row-count -j -k -i input.csv

# Sort by city ascending, then amount descending.
ceslav sort -j -k -b '+s%city,-d%amount' -i input.csv -o sorted.csv

# Headerless input: sort the second column numerically, descending.
ceslav sort -b '-d#1' < input.csv
```

Both commands support:

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

Input uses comma-separated CSV, including quoted fields and quoted newlines. Records must have consistent field counts. Date/time operations, filtering, calculated columns, projection and row selection are planned but not yet implemented.

Errors go to standard error with a nonzero exit code. CSV parse and processing errors identify the record; column errors identify the selector. Output files open only when the command is ready to emit output, so configuration and input errors in these two commands leave existing destinations untouched. Identical input/output files, including hard-link and symlink aliases, are rejected. Output write/flush/close errors are reported; output may be partial after an I/O failure. Redirecting stdout with the shell can truncate a destination before ceslav starts, so use `-o` when file identity checks are needed.

Use `ceslav --help`, `ceslav sort --help` and `ceslav row-count --help` for options. Set `CESLAV_LOG` to a file path to enable diagnostic logging.

Implementation progress is tracked in [docs/implementation-plan.md](docs/implementation-plan.md).
