# Kompa

**Cross-platform developer toolchain manager.**

Kompa installs, updates, and manages compilers, debuggers, and development libraries from a single CLI. Packages are built from official upstream sources by Kompa's GitHub Actions CI and distributed as verified pre-built artifacts through GitHub Releases.

```
kompa install gcc
kompa install clang llvm sqlite lua
kompa update all
kompa env
```

---

## Why Kompa?

Getting a working GCC, LLVM, or Fortran compiler on a fresh machine — or across multiple platforms — involves dozens of steps: finding the right source archive, satisfying build dependencies, compiling for hours, and wiring up environment variables. Kompa automates the entire chain:

- **CI builds from source** — every artifact is compiled from the official upstream release, not a random pre-built binary.
- **Checksum-verified downloads** — SHA-256 is checked before any file is extracted.
- **No system pollution** — all packages land under a single Kompa-managed directory.
- **Clean environment integration** — `kompa env` tells your shell exactly what to export.
- **Version management** — install gcc@13 and gcc@14 side-by-side; switch with `kompa use`.

---

## Installation

### Download a pre-built binary

| Platform | Architecture | Download |
|---|---|---|
| Linux | amd64 | `kompa-linux-amd64` |
| Linux | arm64 | `kompa-linux-arm64` |
| Linux | 386 | `kompa-linux-386` |
| macOS | amd64 | `kompa-darwin-amd64` |
| macOS | arm64 (Apple Silicon) | `kompa-darwin-arm64` |
| Windows | amd64 | `kompa-windows-amd64.exe` |
| Windows | arm64 | `kompa-windows-arm64.exe` |

**Linux / macOS:**
```bash
# Replace VERSION with the latest tag, e.g. v1
curl -Lo kompa https://github.com/kompa-tbm/kompa/releases/download/VERSION/kompa-linux-amd64
chmod +x kompa
sudo mv kompa /usr/local/bin/
```

**Windows (PowerShell):**
```powershell
Invoke-WebRequest -Uri https://github.com/kompa-tbm/kompa/releases/download/VERSION/kompa-windows-amd64.exe -OutFile kompa.exe
# Move kompa.exe to a directory on your PATH
```

### Build from source

Requires Go 1.22+.

```bash
git clone https://github.com/kompa-tbm/kompa.git
cd kompa
go build -o kompa ./cmd/kompa
```

---

## Supported Packages

| Package | Version | Category | Platforms |
|---|---|---|---|
| `gcc` | 14.2.0 | Compiler | Linux, macOS |
| `gfortran` | 14.2.0 | Compiler | Linux, macOS |
| `clang` | 18.1.8 | Compiler | Linux, macOS, Windows |
| `llvm` | 18.1.8 | Toolchain | Linux, macOS, Windows |
| `binutils` | 2.42 | Toolchain | Linux |
| `gdb` | 14.2 | Debugger | Linux, macOS |
| `lldb` | 18.1.8 | Debugger | Linux, macOS |
| `zig` | 0.13.0 | Compiler | Linux, macOS, Windows |
| `go` | 1.22.5 | Compiler | Linux, macOS, Windows |
| `nim` | 2.0.8 | Compiler | Linux, macOS, Windows |
| `ocaml` | 5.2.0 | Compiler | Linux, macOS |
| `ghc` | 9.8.2 | Compiler | Linux, macOS, Windows |
| `ffmpeg` | 7.0.2 | Library | Linux, macOS, Windows |
| `lua` | 5.4.7 | Library | Linux, macOS, Windows |
| `zlib` | 1.3.1 | Library | All |
| `sqlite` | 3.46.1 | Library | All |
| `busybox` | 1.36.1 | Tools | Linux |

Packages marked "All" support all platforms including Windows. Packages with a platform-specific restriction have a detailed explanation accessible via `kompa info <package>`.

---

## CLI Usage

### Installing packages

```bash
# Install the latest version
kompa install gcc

# Install multiple packages at once
kompa install sqlite lua zlib

# Install a specific major version
kompa install gcc@14

# Dependencies are resolved and installed automatically
kompa install clang   # also installs llvm if needed
```

### Removing packages

```bash
kompa remove gcc
kompa uninstall sqlite lua
```

### Updating packages

```bash
# Update a specific package
kompa update gcc

# Update everything
kompa update all
```

### Listing and searching

```bash
# List installed packages
kompa list

# List all installed versions of a package
kompa list gcc

# Search the registry
kompa search compiler
kompa search sqlite
```

### Package information

```bash
kompa info gcc
kompa info sqlite --json
```

### Environment

```bash
# Print a human-readable summary
kompa env

# Print shell export statements
eval "$(kompa env --shell bash)"
eval "$(kompa env --shell zsh)"
kompa env --shell fish | source
kompa env --shell powershell | Invoke-Expression

# Launch a subshell with all packages active
kompa shell
```

### Version management

```bash
# Install two versions side-by-side
kompa install gcc@13
kompa install gcc@14

# Switch the active version
kompa use gcc@14
kompa use gcc@13

# Show the active version
kompa current gcc
kompa current
```

### Diagnostics

```bash
# Check the Kompa installation
kompa doctor
kompa doctor --verbose

# Show version
kompa version
kompa version --json
```

### Cache and configuration

```bash
# Show cache usage
kompa cache info
kompa cache list

# Remove cached downloads (installed packages are unaffected)
kompa clean

# Configuration
kompa config list
kompa config get github_repo
kompa config set github_token ghp_xxxx
kompa config set verbose true

# Update Kompa itself
kompa self-update
```

### Global flags

| Flag | Short | Description |
|---|---|---|
| `--verbose` | `-v` | Enable verbose output |
| `--quiet` | `-q` | Suppress non-essential output |
| `--yes` | `-y` | Auto-confirm prompts |
| `--force` | | Force reinstall / overwrite |
| `--json` | | Output as JSON |
| `--no-cache` | | Bypass cached downloads |

---

## Installation Layout

Kompa manages a self-contained directory — it never scatters files across your system.

| Platform | Default location |
|---|---|
| Linux | `~/.local/share/kompa/` |
| macOS | `~/Library/Application Support/Kompa/` |
| Windows | `%LOCALAPPDATA%\Kompa\` |

Override with the `KOMPA_HOME` environment variable.

```
$KOMPA_HOME/
  bin/          # shims/symlinks for installed binaries
  packages/     # installed package trees (name/version/)
  downloads/    # cached archive files
  cache/        # HTTP metadata cache
  metadata/     # remote release metadata
  manifests/    # local manifest copies
  versions/     # version-selection state
  db.json       # package database
  config.json   # user configuration
```

---

## Architecture

```
Official upstream source
         ↓
  GitHub Actions CI
         ↓
 Build from source
         ↓
Package as tar.zst / zip
         ↓
  Generate manifest.json
     (name, version, sha256, URL)
         ↓
  Create GitHub Release
         ↓
    kompa install
         ↓
  Download artifact
         ↓
  Verify SHA-256
         ↓
    Extract
         ↓
  Write to packages/
         ↓
  Update db.json
         ↓
  Create bin/ shims
```

### Package registry

Package definitions live in `internal/packages/registry/*.json` — one file per package. The JSON schema is versioned (`schema_version`) for forward compatibility.

Each definition records:

- Name, version, description, homepage, license
- Upstream source URL and expected SHA-256
- Build system (`autoconf`, `cmake`, `make`, `custom`, `prebuilt`)
- Configure flags and make targets
- Runtime dependencies
- Supported platform list (with explicit unsupported reasons)
- Installed binaries, libraries, and headers
- Runtime environment variables

### Release manifest

After each CI build, a `manifest.json` is generated and attached to the GitHub Release. The Kompa CLI fetches this manifest to discover available packages and their download URLs.

```json
{
  "schema_version": 1,
  "release_tag": "v7",
  "build_time": "2026-09-04T02:00:00Z",
  "packages": [
    {
      "name": "gcc",
      "version": "14.2.0",
      "os": "linux",
      "architecture": "amd64",
      "archive": "gcc-linux-amd64.tar.zst",
      "sha256": "abc123...",
      "download_url": "https://github.com/kompa-tbm/kompa/releases/download/v7/gcc-linux-amd64.tar.zst",
      "dependencies": ["binutils"]
    }
  ]
}
```

### Dependency resolution

Kompa performs a depth-first topological sort of the dependency graph before installing. Circular dependencies are detected and reported. Already-installed dependencies are skipped. Reverse-dependency warnings are shown on removal.

### Security

- All downloads use HTTPS exclusively.
- SHA-256 checksums are verified before any archive is extracted.
- Archive extraction prevents path traversal attacks: no entry may escape the package's installation directory.
- Absolute symlink targets and symlinks pointing outside the install directory are rejected.
- GitHub tokens are never logged.

---

## CI / Build System

### Workflows

| Workflow | Trigger | Purpose |
|---|---|---|
| `test.yml` | Push / PR | Run `go vet` and `go test` on Linux, macOS, Windows |
| `release.yml` | Push to `main` | Build Kompa CLI for all platforms, create sequential vN release |
| `build-toolchains.yml` | Weekly / manual | Build toolchain packages from upstream source, create release |
| `build-kompa.yml` | Tag push | Build and attach CLI binaries to a tagged release |

### Sequential release tags

Every push to `main` creates the next sequential tag: `v1`, `v2`, `v3`, …

The workflow safely computes `max(existing_vN_tags) + 1`, preventing collisions in concurrent runs.

### Adding a new package to CI

1. Add a JSON definition to `internal/packages/registry/<name>.json`.
2. Add a matrix entry to `.github/workflows/build-toolchains.yml` for each supported platform.
3. The CI will automatically include the new package in the next release.

---

## Adding a New Package Definition

Create `internal/packages/registry/mypkg.json`:

```json
{
  "schema_version": 1,
  "name": "mypkg",
  "version": "1.0.0",
  "description": "My package description",
  "homepage": "https://mypkg.example.com",
  "license": "MIT",
  "source_url": "https://mypkg.example.com/releases/mypkg-1.0.0.tar.gz",
  "source_tag": "v1.0.0",
  "source_sha256": "<sha256 of the source archive>",
  "build_system": "autoconf",
  "configure_args": ["--enable-shared", "--disable-debug"],
  "make_targets": ["all", "install"],
  "artifact_format": "tar.zst",
  "dependencies": [],
  "supported_platforms": [
    {"os": "linux",  "arch": "amd64"},
    {"os": "darwin", "arch": "arm64"}
  ],
  "binaries": ["mypkg"],
  "tags": ["tools"]
}
```

Then add a matrix entry in `.github/workflows/build-toolchains.yml`. That's it — Kompa will pick it up automatically.

---

## Building Kompa Locally

```bash
# Build for the current platform
go build -o kompa ./cmd/kompa

# Cross-compile for Linux arm64
GOOS=linux GOARCH=arm64 go build -o kompa-linux-arm64 ./cmd/kompa

# Cross-compile for Windows
GOOS=windows GOARCH=amd64 go build -o kompa.exe ./cmd/kompa

# Run tests
go test ./...

# Run tests with race detector
go test -race ./...

# Run go vet
go vet ./...

# Inject version at build time
go build -ldflags "-X github.com/kompa-tbm/kompa/internal/cli.Version=1.2.3" ./cmd/kompa
```

---

## Platform / Architecture Compatibility Matrix

| OS | amd64 | arm64 | 386 | riscv64 |
|---|---|---|---|---|
| Linux | ✓ | ✓ | ✓ | planned |
| macOS | ✓ | ✓ | — | — |
| Windows | ✓ | ✓ | — | — |

Kompa itself is a pure-Go binary with no CGo, so it can be cross-compiled for any platform Go supports by setting `GOOS` and `GOARCH`.

---

## Development

```bash
git clone https://github.com/kompa-tbm/kompa.git
cd kompa
go mod download
go test ./...
go build ./cmd/kompa
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for contribution guidelines and [SECURITY.md](SECURITY.md) for the security policy.

---

## License

[MIT](LICENSE)
