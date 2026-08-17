# Contributing

Four rules are not discoverable from the code and will fail your build if you
miss them.

## Zero third-party dependencies

`go list -m all` returns only this module, and that is deliberate. untrace ships
as a single static binary that people run over their whole repository, often in
a pre-commit hook. Every dependency is something they did not choose to trust.

If a change seems to need one, prefer writing or vendoring the few functions
actually required, with the upstream licence and a note in that package's
README.

## Build test fixtures from codepoints, never literals

```go
zwsp := string(rune(0x200B))          // yes
zwsp := "<pasted zero-width space>"   // no
```

This paragraph cannot show you the second form, because writing it would make
this file fail the scan. That is the point: invisible characters are unreadable
in source, Go rejects a literal U+FEFF outright, and a pasted homoglyph makes
the repository flag its own documentation. The fixtures in
`internal/detect/detect_test.go` show the pattern to follow.

CI runs `untrace --fail .` over this repository. `.untrace.json` excludes the
few files that must contain examples.

## Golden files are reviewed, never rubber-stamped

The integration suite under `testdata/cases/` drives the real binary and
compares stdout, stderr, exit code and resulting file bytes.

`go test . -update` regenerates them. It records whatever the code did,
including a bug, so read the diff and confirm the new behaviour is what you
intended before accepting it. A golden diff in a change that was supposed to be
behaviour-neutral means something leaked.

## Allocation ceilings fail the build

`alloc_test.go` in each package asserts that scanning clean input allocates
nothing, and that allocation does not grow with input length or marker count.
See [`docs/design/performance.md`](docs/design/performance.md) for the shapes
that break them.

These are invariants, not measurements: they do not pin a number, so they will
not fight you when detection logic legitimately changes what it finds.

## Running things

```
go test ./...                                    # unit, integration, ceilings
go test ./internal/... -bench . -benchmem        # measurements
go test ./internal/detect/ -fuzz FuzzCleanIsIdempotent -fuzztime 60s
go run ./cmd/untrace --fail .                    # the self-scan CI runs
```

## Design docs

- [`docs/design/resolvers.md`](docs/design/resolvers.md): why the same character
  is legitimate in one file and an attack in another. Read this before touching
  detection.
- [`docs/design/watermark-techniques.md`](docs/design/watermark-techniques.md):
  the landscape and what is reachable at all.
- [`docs/design/performance.md`](docs/design/performance.md): the allocation
  invariants.

## Comments

The bar is high: a comment earns its place only by recording a hidden
constraint, an invariant the types cannot express, a workaround for an external
bug, units or encoding, or a citation. Everything else gets deleted, including
explanations of why a change was made. That belongs in the commit message.
