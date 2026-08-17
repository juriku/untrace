# internal/gitignore

`pattern.go`, `matcher.go` and their tests are copied verbatim from
[go-git](https://github.com/go-git/go-git) v5.19.2,
`plumbing/format/gitignore`, and are licensed under Apache 2.0. The full licence
text is in `LICENSE` in this directory.

They are vendored rather than imported because importing the upstream package
compiles in 13 third-party packages, including an INI parser and
`golang.org/x/net`, to do what these two stdlib-only files already do. untrace
ships as a single binary with no third-party dependencies.

Reading `.gitignore` files from disk is done by `../scan`, replacing upstream's
`dir.go`, which is the file that required the filesystem abstraction.
