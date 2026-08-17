# Allocation invariants

untrace scans whole repositories, so the cost that matters is what an *ordinary*
file pays. Most files contain nothing to find, and that case must stay close to
free.

## The rule

**Allocation must not scale with input length.**

A scan allocates a fixed number of buffers per file, not one per character. The
`[]rune` conversion is the floor and is unavoidable while the payload decoder
and the script census both take `[]rune`. On ASCII that is 4 bytes per input
byte, in three allocations.

Several paths must allocate nothing at all:

| path | invariant |
|---|---|
| `script.MixedWords` on single-script text | zero |
| `decode.Payloads` with no carrier characters | zero |
| `decode.Payloads` on runs below the minimum length | zero |
| `gitignore.Matcher.Match` | zero |
| `textfile.IsBinary` | zero |
| `media.Detect`, `docmeta.Detect` | zero |

These are enforced by `alloc_test.go` in each package, and they fail the build
rather than merely reporting. `bench_test.go` alongside them measures; the
ceilings are what protect the property.

## Why the ceilings are loose

The tests assert design properties, not measured numbers. Pinning a benchmark's
current allocation count would fail whenever detection logic legitimately
changes what it finds, which teaches the reader to ignore the test. A ceiling
that fires on correct work is worse than no ceiling.

So the assertions are of two kinds only: exact zero where the path must not
allocate, and "does not grow with input" where it must not scale. Both survive
changes to what counts as a finding.

## Shapes to avoid

Four mistakes account for every regression found so far, and all four look
harmless in review:

**A parallel array per rune.** A `[]string` or `[]int` sized to the input costs
16 or 8 bytes per character. Track a byte offset and reslice the original string
instead; a substring shares its backing array and allocates nothing.

**A map used as a set of offsets.** A `map[int]bool` spends roughly 50 bytes to
record one bit. Use a bitset, left nil when empty.

**Building a large slice through plain `append`.** Go grows a large slice by
about 1.25x, so the copies total roughly five times the final size. Doubling
manually costs two. See `appendFinding` and `appendWord`.

**Allocating before the test that rejects the work.** The script census built two
maps per word before checking whether the word mixed scripts at all, which is
false for almost every word ever scanned. Decide first, allocate second.

## Measuring

```
go test ./internal/... -bench . -benchmem
go test ./internal/detect/ -bench BenchmarkRunPayloadHeavy -memprofile=/tmp/p.prof
go tool pprof -alloc_space -list 'Run$' /tmp/p.prof
```

Benchmark corpora are 1 MiB, so dividing `B/op` by 1048576 reads as bytes
allocated per byte scanned. `BenchmarkRunBySize` runs the same corpus at four
sizes: its MB/s must stay flat, since a falling figure is quadratic behaviour
rather than a constant factor.
