# Security Policy

## Supported versions

| Version | Supported |
|---|---|
| Latest release | ✓ |
| Older releases | ✗ |

## Reporting a vulnerability

**Do not open a public GitHub issue for security vulnerabilities.**

Please report security issues by emailing the maintainers. Include:
- A description of the vulnerability
- Steps to reproduce
- Potential impact
- Any suggested mitigations

You will receive a response within 72 hours. We will coordinate a fix and disclosure timeline with you.

## Security design

### Download integrity

Every artifact downloaded by Kompa is verified against a SHA-256 checksum recorded in the release manifest before it is extracted. Kompa refuses to install any artifact whose checksum does not match.

### HTTPS only

Kompa refuses to download from non-HTTPS URLs. All GitHub API calls and artifact downloads use HTTPS.

### Archive extraction safety

Kompa's extractor validates every entry in a tar or zip archive before writing it to disk:

- **Path traversal**: entries whose resolved path would fall outside the installation directory are rejected with an error.
- **Absolute symlinks**: symlinks with absolute targets pointing outside the installation directory are rejected.
- **Relative symlink escapes**: relative symlinks that resolve to a path outside the installation directory are rejected.
- **File size limits**: a 4 GiB per-file guard prevents zip/tar bombs from filling disk.

### Manifest trust

The release manifest (`manifest.json`) is fetched from a GitHub Release owned by `kompa-tbm/kompa`. The manifest itself is not cryptographically signed beyond HTTPS transport security. Users who require a higher assurance level should verify individual artifact checksums independently.

### Permissions

Installed binaries and libraries are written with standard user-level permissions (0755/0644). Kompa does not require elevated privileges.

### GitHub token handling

If a GitHub token is configured (`kompa config set github_token …` or `GITHUB_TOKEN` env var), it is used only in `Authorization: Bearer` HTTP headers to the GitHub API. It is never written to log output.
