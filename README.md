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
# Reorder named columns, keeping the header.
ceslav select 'city,name' -jk -i input.csv

# Select by position from headerless input.
ceslav select '#2,#0-1' -i input.csv

# Keep a copy of id under a second name.
ceslav select 'id,id=original_id,city' -j -i input.csv

ceslav drop 'debug,temporary' -jk -i input.csv
```

**Rows start at 1, columns start at `#0`.** A comma-separated column list accepts names, positions and inclusive ranges such as `#0-2` or `#2-0`. Names require `-j`. `select` follows list order. `drop` keeps the remaining columns in input order, removes repeated or overlapping matches once, and requires at least one column to remain.

Use `source=alias` with `select -j` to rename an output column and print the header. Sources always refer to the input header. You can select a source more than once by giving each copy a distinct output name. An alias must be nonempty and unique in the output header. An alias applies to one column.

**Exact names.** The list trims surrounding whitespace. Escape commas, equals signs and backslashes in names with `\,`, `\=` and `\\`. Prefix names beginning with `#` or `%` with `%`. Use `--column '% unit,price '` for a literal name with significant whitespace. Supply one `--column` per selector in place of the list. Use `--` before a list that starts with `-`, after the flags.

## Row selection and removal

```sh
ceslav rows --first 10 -jk -i input.csv
ceslav rows --last 10 --drop -jk -i input.csv
ceslav rows '1,3-4,7' -jk -i input.csv
ceslav rows '7,4-3,1' -jk -i input.csv
ceslav rows '2-4*2' -jk -i input.csv
ceslav rows --reverse -jk -i input.csv
```

Choose a list, `--first N`, or `--last N`. The header is excluded from row numbers and counts. Counts are decimal. Keeping zero emits only the requested header. Dropping zero keeps all data. If a requested row is absent, it contributes no output.

Rows come out in selector order. `3*4` emits row 3 four times, and `2-4*2` emits `2,3,4,2,3,4`. Overlapping selectors repeat matching rows. With `--drop`, each matching row is removed once. `--reverse` reverses the result while leaving any header first. On its own it reverses all data rows.

Forward, disjoint selections stream when their output follows input order. Reordered selections and repeated multirow blocks hold matching rows until selection is complete. A prefix or row list can finish before EOF and skips validation of later records. Keeping the last N rows waits for EOF and holds up to N rows. Dropping the last N rows delays data output by N rows. Reversing all rows holds the full input in memory. Repetition can produce enough output to fill the destination even with a small input.

## Input and output

Input uses comma-separated CSV, including quoted fields and quoted newlines. Records must have consistent field counts. The [implementation goals](docs/implementation-plan.md) track planned date and time operations, filtering, and calculated columns.

Errors go to standard error with a nonzero exit code. Commands open `-o` when they first emit a record or finish successfully with empty output. Buffered row selections and reversals wait to emit their header until the result is ready. Streaming transforms can leave partial output after a later input or write error.

Use `-o` to protect the input file from shell truncation. Ceslav rejects the same input and output file, including hard-link and symlink aliases and a regular file connected to stdout. Shell `>` can truncate a file before ceslav starts.

Run `ceslav COMMAND --help` for usage and examples. Set `CESLAV_LOG` to a file path to enable diagnostic logging.
