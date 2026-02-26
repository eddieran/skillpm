# Security Scanning

> [Docs Index](index.md)

Every skill is scanned before installation and upgrade. The built-in scanner runs six rule categories against skill content and ancillary files.

## Rules

| Rule ID | What It Detects | Default Severity |
|---------|----------------|-----------------|
| `SCAN_DANGEROUS_PATTERN` | `rm -rf /`, `curl\|bash`, reverse shells, credential reads, crypto mining, eval, SSH key exfiltration | Critical / High / Medium |
| `SCAN_PROMPT_INJECTION` | Instruction overrides ("ignore previous instructions"), Unicode tricks (zero-width chars, RTL override), concealment instructions, large encoded blocks | High / Medium |
| `SCAN_FILE_TYPE` | ELF/Mach-O/PE binaries, shared libraries (`.so`, `.dylib`, `.dll`), shell scripts with network commands | High / Medium / Low |
| `SCAN_SIZE_ANOMALY` | SKILL.md > 100KB, single file > 500KB, total files > 5MB, > 50 ancillary files | Medium / Low |
| `SCAN_ENTROPY` | Base64 blocks > 500 chars, hex blocks > 200 chars, multiple high-entropy strings (Shannon > 5.5) | High / Medium |
| `SCAN_NETWORK_INDICATOR` | Hardcoded IP addresses, URL shorteners, non-standard ports, > 5 unique external domains | High / Medium |

## Severity Levels

| Severity | Value | Description |
|----------|-------|-------------|
| Critical | 4 | Always blocks, even with `--force` |
| High | 3 | Blocks by default |
| Medium | 2 | Blocks unless `--force` is passed |
| Low | 1 | Logged, never blocks |
| Info | 0 | Informational only |

## Enforcement Matrix

| Finding Severity | Default Behavior | With `--force` |
|-----------------|-----------------|----------------|
| Critical | **Blocked** | **Blocked** (cannot bypass) |
| High | **Blocked** | Allowed |
| Medium | **Blocked** | Allowed |
| Low | Logged | Logged |
| Info | Logged | Logged |

The block threshold is configurable via `block_severity` in config. The default is `"high"`, meaning high and critical findings block. Setting it to `"medium"` would also block medium findings without `--force`.

## Configuration

In `~/.skillpm/config.toml`:

```toml
[security.scan]
enabled = true              # set to false to disable scanning entirely
block_severity = "high"     # minimum severity that blocks: critical, high, medium, low, info
disabled_rules = []         # rule IDs to skip, e.g. ["SCAN_DANGEROUS_PATTERN"]
```

## Examples

### Blocked install

```bash
$ skillpm install my-repo/suspicious-skill
SEC_SCAN_BLOCKED: [HIGH] SCAN_DANGEROUS_PATTERN (SKILL.md: Code execution via subprocess.run); use --force to proceed
```

### Bypass medium findings

```bash
$ skillpm install my-repo/admin-tool --force
installed admin-tool@v1.2.0
```

### Disable a specific rule

```toml
[security.scan]
disabled_rules = ["SCAN_NETWORK_INDICATOR"]
```

### Disable scanning entirely (not recommended)

```toml
[security.scan]
enabled = false
```

## Integration Points

Scanning runs automatically during:
- `skillpm install` — scans each skill before committing to disk
- `skillpm upgrade` — scans upgraded content before replacing
- `skillpm sync` — scans skills during the upgrade phase

The scanner is not invoked during `inject`, `doctor`, or `list` operations.

## Dangerous Pattern Details

The `SCAN_DANGEROUS_PATTERN` rule checks for:

**Critical patterns:**
- Destructive file deletion (`rm -rf /`, `rm -rf ~/`, `rm -rf $HOME`)
- Remote code execution pipes (`curl ... | sh`, `wget ... | sh`)
- Obfuscated execution (`base64 -d | sh`)
- Sensitive file access (`/etc/shadow`, `/etc/passwd`)
- Reverse shells (`mkfifo ... nc`, `nc -e /bin/`)
- SSH key exfiltration (`~/.ssh/id_rsa`)
- Crypto mining indicators (`stratum+tcp://`, `xmrig`, `minerd`)
- Dangerous permissions (`chmod 777 /`)
- Arbitrary code execution (`eval(`)

**High patterns:**
- Code execution in non-shell contexts (`os.exec`, `subprocess.run`, `child_process.exec`)
- Environment variable harvesting (`os.environ`)
- Data exfiltration (`curl -d`, `wget --post-data`)
- Credential file references (`.env`, `credentials.json`, `secrets.yaml`)
- Global git config modification
- Package installation commands (`pip install`, `npm install`)

**Medium patterns:**
- `sudo` usage
