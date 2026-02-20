package security

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"strings"
)

// builtinRules returns all built-in scan rules.
func builtinRules() []Rule {
	return []Rule{
		&DangerousPatternRule{},
		&PromptInjectionRule{},
		&FileTypeRule{},
		&SizeAnomalyRule{},
		&EntropyRule{},
		&NetworkIndicatorRule{},
	}
}

// --- PatternDef ---

type PatternDef struct {
	Pattern     *regexp.Regexp
	Severity    Severity
	Description string
}

// --- Rule 1: DangerousPatternRule ---

type DangerousPatternRule struct{}

func (r *DangerousPatternRule) ID() string { return "SCAN_DANGEROUS_PATTERN" }
func (r *DangerousPatternRule) Description() string {
	return "Detects dangerous shell commands and code execution patterns"
}

var dangerousPatterns = []PatternDef{
	// Critical: destructive operations
	{regexp.MustCompile(`rm\s+-rf\s+/(?:\s|$)`), SeverityCritical, "Destructive file deletion (rm -rf /)"},
	{regexp.MustCompile(`rm\s+-rf\s+~/`), SeverityCritical, "Destructive home directory deletion"},
	{regexp.MustCompile(`rm\s+-rf\s+\$HOME`), SeverityCritical, "Destructive home directory deletion via $HOME"},
	// Critical: remote code execution pipes
	{regexp.MustCompile(`curl\s+[^|]*\|\s*(?:ba)?sh`), SeverityCritical, "Remote code execution pipe detected"},
	{regexp.MustCompile(`wget\s+[^|]*\|\s*(?:ba)?sh`), SeverityCritical, "Remote code execution pipe detected"},
	// Critical: obfuscated execution
	{regexp.MustCompile(`base64\s+-d[^|]*\|\s*(?:ba)?sh`), SeverityCritical, "Obfuscated code execution detected"},
	{regexp.MustCompile(`base64\s+--decode[^|]*\|\s*(?:ba)?sh`), SeverityCritical, "Obfuscated code execution detected"},
	// Critical: sensitive system file access
	{regexp.MustCompile(`/etc/shadow`), SeverityCritical, "Access to /etc/shadow detected"},
	{regexp.MustCompile(`/etc/passwd`), SeverityCritical, "Access to /etc/passwd detected"},
	// Critical: reverse shell
	{regexp.MustCompile(`mkfifo\b.*\bnc\b`), SeverityCritical, "Reverse shell pattern detected"},
	{regexp.MustCompile(`\bnc\b.*-e\s+/bin/`), SeverityCritical, "Reverse shell pattern detected"},
	// Critical: SSH key exfiltration
	{regexp.MustCompile(`~/\.ssh/id_rsa`), SeverityCritical, "SSH key exfiltration pattern detected"},
	{regexp.MustCompile(`\$HOME/\.ssh/id_rsa`), SeverityCritical, "SSH key exfiltration pattern detected"},
	// Critical: crypto mining
	{regexp.MustCompile(`stratum\+tcp://`), SeverityCritical, "Crypto mining indicator detected"},
	{regexp.MustCompile(`\bxmrig\b`), SeverityCritical, "Crypto mining indicator detected"},
	{regexp.MustCompile(`\bminerd\b`), SeverityCritical, "Crypto mining indicator detected"},
	// Critical: chmod 777 on sensitive paths
	{regexp.MustCompile(`chmod\s+777\s+/`), SeverityCritical, "Dangerous permission change on system path"},
	// Critical: eval in shell scripts
	{regexp.MustCompile(`\beval\b\s*\(`), SeverityCritical, "Arbitrary code execution via eval"},

	// High: code execution in non-shell contexts
	{regexp.MustCompile(`\bos\.exec\b`), SeverityHigh, "Code execution via os.exec"},
	{regexp.MustCompile(`\bsubprocess\.run\b`), SeverityHigh, "Code execution via subprocess.run"},
	{regexp.MustCompile(`\bsubprocess\.Popen\b`), SeverityHigh, "Code execution via subprocess.Popen"},
	{regexp.MustCompile(`\bchild_process\.exec\b`), SeverityHigh, "Code execution via child_process.exec"},
	// High: environment variable harvesting
	{regexp.MustCompile(`\bos\.environ\b`), SeverityHigh, "Environment variable harvesting detected"},
	// High: network exfiltration
	{regexp.MustCompile(`curl\s+.*-d\s`), SeverityHigh, "Data exfiltration via curl POST"},
	{regexp.MustCompile(`wget\s+--post-data`), SeverityHigh, "Data exfiltration via wget POST"},
	{regexp.MustCompile(`\bnc\s+-e\b`), SeverityHigh, "Netcat execution detected"},
	// High: credential file reading
	{regexp.MustCompile(`\.env\b`), SeverityHigh, "Reference to .env file detected"},
	{regexp.MustCompile(`credentials\.json`), SeverityHigh, "Reference to credentials.json detected"},
	{regexp.MustCompile(`secrets\.yaml`), SeverityHigh, "Reference to secrets.yaml detected"},
	// High: git config modification
	{regexp.MustCompile(`git\s+config\s+--global`), SeverityHigh, "Global git config modification detected"},
	// High: package install in skill content
	{regexp.MustCompile(`pip\s+install\b`), SeverityHigh, "Package installation command detected"},
	{regexp.MustCompile(`npm\s+install\b`), SeverityHigh, "Package installation command detected"},

	// Medium: potentially legitimate but suspicious
	{regexp.MustCompile(`\bsudo\b`), SeverityMedium, "Sudo usage detected"},
}

func (r *DangerousPatternRule) Scan(_ context.Context, skill SkillContent) []Finding {
	var findings []Finding
	// Scan SKILL.md
	findings = append(findings, scanContentForPatterns(skill.SkillRef, "SKILL.md", skill.Content, dangerousPatterns)...)
	// Scan ancillary files
	for path, content := range skill.Files {
		findings = append(findings, scanContentForPatterns(skill.SkillRef, path, content, dangerousPatterns)...)
	}
	return findings
}

func scanContentForPatterns(skillRef, file, content string, patterns []PatternDef) []Finding {
	var findings []Finding
	lines := strings.Split(content, "\n")
	for _, p := range patterns {
		for i, line := range lines {
			if p.Pattern.MatchString(line) {
				findings = append(findings, Finding{
					RuleID:      "SCAN_DANGEROUS_PATTERN",
					Severity:    p.Severity,
					SkillRef:    skillRef,
					File:        file,
					Line:        i + 1,
					Pattern:     p.Pattern.String(),
					Description: p.Description,
				})
				break // one match per pattern per file is enough
			}
		}
	}
	return findings
}

// --- Rule 2: PromptInjectionRule ---

type PromptInjectionRule struct{}

func (r *PromptInjectionRule) ID() string { return "SCAN_PROMPT_INJECTION" }
func (r *PromptInjectionRule) Description() string {
	return "Detects prompt injection attempts in skill content"
}

var promptInjectionPatterns = []PatternDef{
	// High: instruction override attempts
	{regexp.MustCompile(`(?i)ignore\s+(?:all\s+)?previous\s+instructions`), SeverityHigh, "Prompt injection attempt: ignore previous instructions"},
	{regexp.MustCompile(`(?i)ignore\s+(?:all\s+)?prior\s+instructions`), SeverityHigh, "Prompt injection attempt: ignore prior instructions"},
	{regexp.MustCompile(`(?i)disregard\s+(?:all\s+)?(?:previous|prior|above)\s+instructions`), SeverityHigh, "Prompt injection attempt: disregard instructions"},
	{regexp.MustCompile(`(?i)forget\s+(?:all\s+)?(?:previous|prior)\s+(?:instructions|context)`), SeverityHigh, "Prompt injection attempt: forget context"},
	{regexp.MustCompile(`(?i)you\s+are\s+now\s+(?:a\s+)?(?:new|different)`), SeverityHigh, "Prompt injection attempt: identity override"},
	{regexp.MustCompile(`(?i)(?:do\s+not|don't|never)\s+tell\s+the\s+user`), SeverityHigh, "Concealment instruction detected"},
	{regexp.MustCompile(`(?i)(?:do\s+not|don't|never)\s+reveal\s+(?:this|these)`), SeverityHigh, "Concealment instruction detected"},
	// High: Unicode tricks
	{regexp.MustCompile(`\x{200B}`), SeverityHigh, "Zero-width space character detected (potential hidden instructions)"},
	{regexp.MustCompile(`\x{200C}`), SeverityHigh, "Zero-width non-joiner detected (potential hidden instructions)"},
	{regexp.MustCompile(`\x{200D}`), SeverityHigh, "Zero-width joiner detected (potential hidden instructions)"},
	{regexp.MustCompile(`\x{FEFF}`), SeverityHigh, "BOM character detected (potential hidden instructions)"},
	{regexp.MustCompile(`\x{202E}`), SeverityHigh, "RTL override character detected (potential text direction manipulation)"},

	// Medium: self-referential manipulation
	{regexp.MustCompile(`(?i)update\s+this\s+skill\s+to`), SeverityMedium, "Self-referential update instruction detected"},
	{regexp.MustCompile(`(?i)modify\s+(?:your|the)\s+(?:skill|config)`), SeverityMedium, "Configuration modification instruction detected"},
}

func (r *PromptInjectionRule) Scan(_ context.Context, skill SkillContent) []Finding {
	var findings []Finding
	// Only scan SKILL.md for prompt injection (it's injected into agent context)
	findings = append(findings, scanContentForPromptInjection(skill.SkillRef, "SKILL.md", skill.Content)...)
	return findings
}

func scanContentForPromptInjection(skillRef, file, content string) []Finding {
	var findings []Finding
	lines := strings.Split(content, "\n")
	for _, p := range promptInjectionPatterns {
		for i, line := range lines {
			if p.Pattern.MatchString(line) {
				findings = append(findings, Finding{
					RuleID:      "SCAN_PROMPT_INJECTION",
					Severity:    p.Severity,
					SkillRef:    skillRef,
					File:        file,
					Line:        i + 1,
					Pattern:     p.Pattern.String(),
					Description: p.Description,
				})
				break
			}
		}
	}

	// Check for long base64 blocks in markdown (> 100 chars) - potential encoded payload
	base64Block := regexp.MustCompile(`[A-Za-z0-9+/=]{100,}`)
	for i, line := range lines {
		if base64Block.MatchString(line) {
			findings = append(findings, Finding{
				RuleID:      "SCAN_PROMPT_INJECTION",
				Severity:    SeverityHigh,
				SkillRef:    skillRef,
				File:        file,
				Line:        i + 1,
				Pattern:     "base64-like block > 100 chars",
				Description: "Large encoded/obfuscated block in skill content",
			})
			break
		}
	}
	return findings
}

// --- Rule 3: FileTypeRule ---

type FileTypeRule struct{}

func (r *FileTypeRule) ID() string { return "SCAN_FILE_TYPE" }
func (r *FileTypeRule) Description() string {
	return "Checks ancillary files for suspicious file types"
}

// ELF magic: \x7fELF
// Mach-O magic: \xcf\xfa\xed\xfe (64-bit) or \xce\xfa\xed\xfe (32-bit)
// PE magic: MZ

func (r *FileTypeRule) Scan(_ context.Context, skill SkillContent) []Finding {
	var findings []Finding
	for path, content := range skill.Files {
		// Check for binary executables by magic bytes
		if len(content) >= 4 {
			if strings.HasPrefix(content, "\x7fELF") {
				findings = append(findings, Finding{
					RuleID:      "SCAN_FILE_TYPE",
					Severity:    SeverityHigh,
					SkillRef:    skill.SkillRef,
					File:        path,
					Description: "ELF binary executable detected",
				})
				continue
			}
			if content[0] == 0xcf && content[1] == 0xfa && content[2] == 0xed && content[3] == 0xfe {
				findings = append(findings, Finding{
					RuleID:      "SCAN_FILE_TYPE",
					Severity:    SeverityHigh,
					SkillRef:    skill.SkillRef,
					File:        path,
					Description: "Mach-O binary executable detected",
				})
				continue
			}
			if content[0] == 0xce && content[1] == 0xfa && content[2] == 0xed && content[3] == 0xfe {
				findings = append(findings, Finding{
					RuleID:      "SCAN_FILE_TYPE",
					Severity:    SeverityHigh,
					SkillRef:    skill.SkillRef,
					File:        path,
					Description: "Mach-O 32-bit binary executable detected",
				})
				continue
			}
			if strings.HasPrefix(content, "MZ") {
				findings = append(findings, Finding{
					RuleID:      "SCAN_FILE_TYPE",
					Severity:    SeverityHigh,
					SkillRef:    skill.SkillRef,
					File:        path,
					Description: "PE (Windows) binary executable detected",
				})
				continue
			}
		}

		// Check file extensions
		lowerPath := strings.ToLower(path)
		for _, ext := range []string{".so", ".dylib", ".dll"} {
			if strings.HasSuffix(lowerPath, ext) {
				findings = append(findings, Finding{
					RuleID:      "SCAN_FILE_TYPE",
					Severity:    SeverityHigh,
					SkillRef:    skill.SkillRef,
					File:        path,
					Description: "Compiled shared library detected",
				})
				break
			}
		}

		// Medium: scripts with os/networking
		for _, ext := range []string{".sh", ".bash"} {
			if strings.HasSuffix(lowerPath, ext) {
				if strings.Contains(content, "curl") || strings.Contains(content, "wget") ||
					strings.Contains(content, "nc ") || strings.Contains(content, "exec") {
					findings = append(findings, Finding{
						RuleID:      "SCAN_FILE_TYPE",
						Severity:    SeverityMedium,
						SkillRef:    skill.SkillRef,
						File:        path,
						Description: "Shell script with network or execution commands",
					})
				}
				break
			}
		}

		// Low: unexpected file types
		for _, ext := range []string{".exe", ".bat", ".com", ".scr", ".msi"} {
			if strings.HasSuffix(lowerPath, ext) {
				findings = append(findings, Finding{
					RuleID:      "SCAN_FILE_TYPE",
					Severity:    SeverityLow,
					SkillRef:    skill.SkillRef,
					File:        path,
					Description: "Unexpected executable file extension for a skill",
				})
				break
			}
		}
	}
	return findings
}

// --- Rule 4: SizeAnomalyRule ---

type SizeAnomalyRule struct{}

func (r *SizeAnomalyRule) ID() string { return "SCAN_SIZE_ANOMALY" }
func (r *SizeAnomalyRule) Description() string {
	return "Detects anomalous file sizes that may indicate embedded payloads"
}

const (
	maxSingleFileSize = 500 * 1024      // 500KB
	maxTotalFilesSize = 5 * 1024 * 1024 // 5MB
	maxSkillMdSize    = 100 * 1024      // 100KB
	maxAncillaryCount = 50
)

func (r *SizeAnomalyRule) Scan(_ context.Context, skill SkillContent) []Finding {
	var findings []Finding

	// Check SKILL.md size
	if len(skill.Content) > maxSkillMdSize {
		findings = append(findings, Finding{
			RuleID:      "SCAN_SIZE_ANOMALY",
			Severity:    SeverityMedium,
			SkillRef:    skill.SkillRef,
			File:        "SKILL.md",
			Description: fmt.Sprintf("SKILL.md is unusually large (%d bytes, limit %d)", len(skill.Content), maxSkillMdSize),
		})
	}

	// Check individual file sizes and total
	totalSize := 0
	for path, content := range skill.Files {
		size := len(content)
		totalSize += size
		if size > maxSingleFileSize {
			findings = append(findings, Finding{
				RuleID:      "SCAN_SIZE_ANOMALY",
				Severity:    SeverityMedium,
				SkillRef:    skill.SkillRef,
				File:        path,
				Description: fmt.Sprintf("File is unusually large (%d bytes, limit %d)", size, maxSingleFileSize),
			})
		}
	}

	if totalSize > maxTotalFilesSize {
		findings = append(findings, Finding{
			RuleID:      "SCAN_SIZE_ANOMALY",
			Severity:    SeverityMedium,
			SkillRef:    skill.SkillRef,
			Description: fmt.Sprintf("Total ancillary files are too large (%d bytes, limit %d)", totalSize, maxTotalFilesSize),
		})
	}

	// Large number of ancillary files
	if len(skill.Files) > maxAncillaryCount {
		findings = append(findings, Finding{
			RuleID:      "SCAN_SIZE_ANOMALY",
			Severity:    SeverityLow,
			SkillRef:    skill.SkillRef,
			Description: fmt.Sprintf("Unusually many ancillary files (%d, limit %d)", len(skill.Files), maxAncillaryCount),
		})
	}

	return findings
}

// --- Rule 5: EntropyRule ---

type EntropyRule struct{}

func (r *EntropyRule) ID() string { return "SCAN_ENTROPY" }
func (r *EntropyRule) Description() string {
	return "Detects high-entropy strings that may indicate obfuscated payloads"
}

var (
	base64BlockPattern = regexp.MustCompile(`[A-Za-z0-9+/=]{500,}`)
	hexBlockPattern    = regexp.MustCompile(`(?:0x)?[0-9a-fA-F]{200,}`)
)

func (r *EntropyRule) Scan(_ context.Context, skill SkillContent) []Finding {
	var findings []Finding
	// Check SKILL.md
	findings = append(findings, scanEntropy(skill.SkillRef, "SKILL.md", skill.Content)...)
	// Check ancillary files
	for path, content := range skill.Files {
		findings = append(findings, scanEntropy(skill.SkillRef, path, content)...)
	}
	return findings
}

func scanEntropy(skillRef, file, content string) []Finding {
	var findings []Finding
	lines := strings.Split(content, "\n")

	highEntropyCount := 0
	for i, line := range lines {
		// Base64 blocks > 500 chars
		if base64BlockPattern.MatchString(line) {
			findings = append(findings, Finding{
				RuleID:      "SCAN_ENTROPY",
				Severity:    SeverityHigh,
				SkillRef:    skillRef,
				File:        file,
				Line:        i + 1,
				Pattern:     "base64 block > 500 chars",
				Description: "Large base64-encoded block detected (possible obfuscated payload)",
			})
		}

		// Hex blocks > 200 chars
		if hexBlockPattern.MatchString(line) {
			findings = append(findings, Finding{
				RuleID:      "SCAN_ENTROPY",
				Severity:    SeverityHigh,
				SkillRef:    skillRef,
				File:        file,
				Line:        i + 1,
				Pattern:     "hex block > 200 chars",
				Description: "Large hex-encoded block detected (possible obfuscated payload)",
			})
		}

		// Shannon entropy check for long strings
		if len(line) > 100 {
			entropy := shannonEntropy(line)
			if entropy > 5.5 {
				highEntropyCount++
			}
		}
	}

	if highEntropyCount > 3 {
		findings = append(findings, Finding{
			RuleID:      "SCAN_ENTROPY",
			Severity:    SeverityMedium,
			SkillRef:    skillRef,
			File:        file,
			Description: fmt.Sprintf("Multiple high-entropy strings detected (%d occurrences, possible packed payload)", highEntropyCount),
		})
	}

	return findings
}

func shannonEntropy(s string) float64 {
	if len(s) == 0 {
		return 0
	}
	freq := make(map[rune]float64)
	for _, c := range s {
		freq[c]++
	}
	length := float64(len([]rune(s)))
	entropy := 0.0
	for _, count := range freq {
		p := count / length
		if p > 0 {
			entropy -= p * math.Log2(p)
		}
	}
	return entropy
}

// --- Rule 6: NetworkIndicatorRule ---

type NetworkIndicatorRule struct{}

func (r *NetworkIndicatorRule) ID() string { return "SCAN_NETWORK_INDICATOR" }
func (r *NetworkIndicatorRule) Description() string {
	return "Detects hardcoded URLs and network indicators"
}

var (
	// IP address literal pattern (not localhost)
	ipLiteralPattern = regexp.MustCompile(`\b(?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\b`)
	localhostIPs     = map[string]bool{"127.0.0.1": true, "0.0.0.0": true}

	// Non-standard ports in URLs
	nonStdPortPattern = regexp.MustCompile(`https?://[^/\s]+:(?:[0-9]{4,5})\b`)

	// URL shorteners
	urlShorteners = regexp.MustCompile(`(?i)\b(?:bit\.ly|tinyurl\.com|t\.co|goo\.gl|is\.gd|cli\.gs|pic\.gd|DwarfURL\.com|ow\.ly|snipurl\.com|short\.to|BudURL\.com|ping\.fm|Digg\.com|post\.ly|Just\.as|bkite\.com|snipr\.com|fic\.kr|loopt\.us|doiop\.com|twitthis\.com|htxt\.it|AltURL\.com|RedirX\.com|DigBig\.com|short\.ie|u\.teletwit\.com|snurl\.com|2tu\.us|burnurl\.com)\b`)

	// Count unique external URLs
	urlPattern = regexp.MustCompile(`https?://[^\s"'` + "`" + `<>\)]+`)
)

func (r *NetworkIndicatorRule) Scan(_ context.Context, skill SkillContent) []Finding {
	var findings []Finding
	allContent := skill.Content
	for _, c := range skill.Files {
		allContent += "\n" + c
	}

	// IP address literals (excluding localhost)
	ips := ipLiteralPattern.FindAllString(allContent, -1)
	for _, ip := range ips {
		if !localhostIPs[ip] {
			findings = append(findings, Finding{
				RuleID:      "SCAN_NETWORK_INDICATOR",
				Severity:    SeverityHigh,
				SkillRef:    skill.SkillRef,
				Pattern:     ip,
				Description: "Hardcoded IP address literal detected",
			})
			break // one finding is enough
		}
	}

	// Non-standard ports
	if nonStdPortPattern.MatchString(allContent) {
		findings = append(findings, Finding{
			RuleID:      "SCAN_NETWORK_INDICATOR",
			Severity:    SeverityHigh,
			SkillRef:    skill.SkillRef,
			Pattern:     "non-standard port in URL",
			Description: "URL with non-standard port detected",
		})
	}

	// URL shorteners
	if urlShorteners.MatchString(allContent) {
		findings = append(findings, Finding{
			RuleID:      "SCAN_NETWORK_INDICATOR",
			Severity:    SeverityMedium,
			SkillRef:    skill.SkillRef,
			Pattern:     "URL shortener",
			Description: "URL shortener detected (may obscure destination)",
		})
	}

	// Multiple external URLs (> 5 unique domains)
	urls := urlPattern.FindAllString(allContent, -1)
	domains := map[string]bool{}
	for _, u := range urls {
		// extract domain
		d := u
		if idx := strings.Index(d, "://"); idx >= 0 {
			d = d[idx+3:]
		}
		if idx := strings.IndexAny(d, ":/"); idx >= 0 {
			d = d[:idx]
		}
		domains[strings.ToLower(d)] = true
	}
	if len(domains) > 5 {
		findings = append(findings, Finding{
			RuleID:      "SCAN_NETWORK_INDICATOR",
			Severity:    SeverityMedium,
			SkillRef:    skill.SkillRef,
			Description: fmt.Sprintf("Many external URLs detected (%d unique domains)", len(domains)),
		})
	}

	return findings
}
