# Security Policy

## Supported Versions

The `main` branch receives active security updates.

## Reporting a Vulnerability

Please report vulnerabilities privately:

- Open a private security advisory on GitHub (preferred), or
- Email the maintainer listed in repository metadata.

Include:
- Affected version/commit
- Reproduction steps
- Impact assessment
- Suggested mitigation (if available)

We will acknowledge reports as quickly as possible and coordinate responsible disclosure.

## Security Scope

High-priority areas:
- Source authenticity/provenance
- Checksum/signature validation
- Path traversal and symlink escape controls
- Injection/removal rollback safety
- Policy enforcement in strict mode
- Pre-install content scanning (dangerous patterns, prompt injection, binary detection, entropy analysis, network indicators)
