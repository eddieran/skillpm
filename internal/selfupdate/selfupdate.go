package selfupdate

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strings"
	"time"
)

type Manifest struct {
	Version   string `json:"version"`
	URL       string `json:"url"`
	Checksum  string `json:"checksum"`
	Signature string `json:"signature,omitempty"`
	PublicKey string `json:"publicKey,omitempty"`
}

type Result struct {
	Channel    string `json:"channel"`
	Version    string `json:"version"`
	Executable string `json:"executable"`
	Updated    bool   `json:"updated"`
}

type Service struct {
	client *http.Client
}

func New(client *http.Client) *Service {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &Service{client: client}
}

func (s *Service) Update(ctx context.Context, channel string, requireSignatures bool) (Result, error) {
	if channel == "" {
		channel = "stable"
	}
	manifestURL := resolveManifestURL(channel)
	manifest, err := s.fetchManifest(ctx, manifestURL)
	if err != nil {
		return Result{}, err
	}
	if manifest.URL == "" || manifest.Checksum == "" {
		return Result{}, fmt.Errorf("SEC_SELF_UPDATE_MANIFEST: incomplete manifest")
	}
	binary, err := s.fetchBinary(ctx, manifestURL, manifest.URL)
	if err != nil {
		return Result{}, err
	}
	if err := verifyChecksum(binary, manifest.Checksum); err != nil {
		return Result{}, err
	}
	if err := verifySignature(binary, manifest.Signature, manifest.PublicKey, requireSignatures); err != nil {
		return Result{}, err
	}
	exe := os.Getenv("SKILLPM_SELF_UPDATE_TARGET")
	if exe == "" {
		exe, err = os.Executable()
		if err != nil {
			return Result{}, fmt.Errorf("SEC_SELF_UPDATE_EXEC: %w", err)
		}
	}
	if err := applyBinarySwap(exe, binary); err != nil {
		return Result{}, err
	}
	return Result{Channel: channel, Version: manifest.Version, Executable: exe, Updated: true}, nil
}

func resolveManifestURL(channel string) string {
	if explicit := os.Getenv("SKILLPM_UPDATE_MANIFEST_URL"); explicit != "" {
		return explicit
	}
	base := os.Getenv("SKILLPM_UPDATE_MANIFEST_BASE")
	if base == "" {
		base = "https://github.com/eddieran/skillpm/releases/latest/download/"
	}
	if !strings.HasSuffix(base, "/") {
		base += "/"
	}
	name := fmt.Sprintf("manifest-%s-%s-%s.json", channel, runtime.GOOS, runtime.GOARCH)
	return base + name
}

func (s *Service) fetchManifest(ctx context.Context, manifestURL string) (Manifest, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, manifestURL, nil)
	if err != nil {
		return Manifest{}, err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return Manifest{}, fmt.Errorf("SEC_SELF_UPDATE_FETCH: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Manifest{}, fmt.Errorf("SEC_SELF_UPDATE_FETCH: status %d", resp.StatusCode)
	}
	var manifest Manifest
	if err := json.NewDecoder(resp.Body).Decode(&manifest); err != nil {
		return Manifest{}, fmt.Errorf("SEC_SELF_UPDATE_MANIFEST: %w", err)
	}
	return manifest, nil
}

func (s *Service) fetchBinary(ctx context.Context, manifestURL, binaryURL string) ([]byte, error) {
	resolved := binaryURL
	if u, err := url.Parse(binaryURL); err == nil && !u.IsAbs() {
		base, baseErr := url.Parse(manifestURL)
		if baseErr == nil {
			resolved = base.ResolveReference(u).String()
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, resolved, nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("SEC_SELF_UPDATE_DOWNLOAD: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("SEC_SELF_UPDATE_DOWNLOAD: status %d", resp.StatusCode)
	}
	blob, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if len(blob) == 0 {
		return nil, fmt.Errorf("SEC_SELF_UPDATE_DOWNLOAD: empty payload")
	}
	return blob, nil
}

func verifyChecksum(binary []byte, expected string) error {
	expected = strings.TrimSpace(expected)
	expected = strings.TrimPrefix(expected, "sha256:")
	expected = strings.ToLower(expected)
	h := sha256.Sum256(binary)
	actual := hex.EncodeToString(h[:])
	if actual != expected {
		return fmt.Errorf("SEC_SELF_UPDATE_CHECKSUM: expected %s got %s", expected, actual)
	}
	return nil
}

func verifySignature(binary []byte, sigB64, keyB64 string, required bool) error {
	if sigB64 == "" || keyB64 == "" {
		if required {
			return fmt.Errorf("SEC_SELF_UPDATE_SIGNATURE: signature required but missing")
		}
		return nil
	}
	sig, err := base64.StdEncoding.DecodeString(sigB64)
	if err != nil {
		return fmt.Errorf("SEC_SELF_UPDATE_SIGNATURE: invalid signature encoding")
	}
	key, err := base64.StdEncoding.DecodeString(keyB64)
	if err != nil {
		return fmt.Errorf("SEC_SELF_UPDATE_SIGNATURE: invalid public key encoding")
	}
	if len(key) != ed25519.PublicKeySize {
		return fmt.Errorf("SEC_SELF_UPDATE_SIGNATURE: invalid public key size")
	}
	if !ed25519.Verify(ed25519.PublicKey(key), binary, sig) {
		return fmt.Errorf("SEC_SELF_UPDATE_SIGNATURE: signature verification failed")
	}
	return nil
}

func applyBinarySwap(executable string, binary []byte) error {
	mode := os.FileMode(0o755)
	if stat, err := os.Stat(executable); err == nil {
		mode = stat.Mode().Perm()
	}
	newPath := executable + ".new"
	backupPath := executable + ".bak"
	if err := os.WriteFile(newPath, binary, mode); err != nil {
		return fmt.Errorf("SEC_SELF_UPDATE_WRITE: %w", err)
	}
	if err := os.Rename(executable, backupPath); err != nil {
		_ = os.Remove(newPath)
		return fmt.Errorf("SEC_SELF_UPDATE_SWAP: backup failed: %w", err)
	}
	if os.Getenv("SKILLPM_TEST_FAIL_SELF_UPDATE_SWAP") == "1" {
		_ = os.Rename(backupPath, executable)
		_ = os.Remove(newPath)
		return fmt.Errorf("SEC_SELF_UPDATE_SWAP: injected swap failure")
	}
	if err := os.Rename(newPath, executable); err != nil {
		_ = os.Rename(backupPath, executable)
		_ = os.Remove(newPath)
		return fmt.Errorf("SEC_SELF_UPDATE_SWAP: apply failed: %w", err)
	}
	_ = os.Remove(backupPath)
	return nil
}
