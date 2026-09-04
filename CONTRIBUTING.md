# Contributing to Kompa

Thank you for your interest in contributing to Kompa.

## Getting started

1. Fork the repository at https://github.com/kompa-tbm/kompa.
2. Clone your fork: `git clone https://github.com/<you>/kompa.git`
3. Create a feature branch: `git checkout -b feature/my-change`
4. Make your changes, add tests, and ensure everything passes.
5. Push and open a Pull Request against `main`.

## Development setup

```bash
# Requires Go 1.22+
go mod download
go build ./cmd/kompa
go test ./...
go vet ./...
```

## Adding a new package

1. Create `internal/packages/registry/<name>.json` following the schema in `internal/packages/definition.go`.
2. Add the corresponding matrix entry in `.github/workflows/build-toolchains.yml`.
3. Add tests if the package has unusual dependency or platform constraints.
4. Run `go test ./...` to verify the registry loads and validates cleanly.

## Code style

- Follow standard Go conventions (`gofmt`, `go vet`).
- New exported symbols must have doc comments.
- All errors must be wrapped with context using `fmt.Errorf("...: %w", err)`.
- No `panic` for user-visible errors — return them.
- Tests must be in `_test.go` files alongside the package they test.

## Commit messages

Use the conventional format:

```
feat: add mypkg package definition
fix: correct path traversal check on Windows
test: add store SetActive tests
docs: update README installation section
```

## Pull Request checklist

- [ ] `go vet ./...` passes
- [ ] `go test ./...` passes (including race detector: `go test -race ./...`)
- [ ] New packages have a valid JSON definition
- [ ] New features have corresponding tests
- [ ] Documentation updated if user-facing behaviour changed

## Reporting bugs

Open an issue at https://github.com/kompa-tbm/kompa/issues and include:
- Kompa version (`kompa version`)
- Operating system and architecture
- Full command and output
- Expected vs actual behaviour

## Security vulnerabilities

See [SECURITY.md](SECURITY.md).
