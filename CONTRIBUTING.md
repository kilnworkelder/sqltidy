# Contributing to sqltidy

Thanks for helping improve sqltidy!

## Development

- Go 1.26 or newer (`go version` to check).
- `make test` (or `go test -count=1 ./...`) must pass.
- `make vet` (or `go vet ./...`) and `gofmt` must be clean.
- CI runs the test matrix on Linux, macOS, and Windows, plus
  golangci-lint, on every push and pull request.

## Commits

Use [Conventional Commits](https://www.conventionalcommits.org/) prefixes
(`feat:`, `fix:`, `docs:`, `chore:`). GoReleaser derives release changelogs
from these messages.

## Pull requests

1. Branch from `main`.
2. Add or update tests for any behaviour change — table-driven tests live
   next to the code they cover.
3. Keep CLI output formats stable; they are documented in the README and
   relied on by scripts.

## Releases

Maintainers release by tagging:

```console
git tag -a vX.Y.Z -m "vX.Y.Z"
git push origin vX.Y.Z
```

GoReleaser cross-compiles, attaches archives to the GitHub release, and the
module proxy serves the tag for `go install ...@vX.Y.Z`.
