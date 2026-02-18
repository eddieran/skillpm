package selfupdate

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestUpdateAppliesVerifiedBinary(t *testing.T) {
	oldBin := []byte("old-binary")
	newBin := []byte("new-binary")
	target := filepath.Join(t.TempDir(), "skillpm")
	if err := os.WriteFile(target, oldBin, 0o755); err != nil {
		t.Fatalf("write target failed: %v", err)
	}

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key failed: %v", err)
	}
	h := sha256.Sum256(newBin)
	checksum := hex.EncodeToString(h[:])
	sig := ed25519.Sign(priv, newBin)

	mux := http.NewServeMux()
	mux.HandleFunc("/bin", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(newBin)
	})
	mux.HandleFunc("/manifest", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(Manifest{
			Version:   "1.2.3",
			URL:       "/bin",
			Checksum:  checksum,
			Signature: base64.StdEncoding.EncodeToString(sig),
			PublicKey: base64.StdEncoding.EncodeToString(pub),
		})
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	t.Setenv("SKILLPM_SELF_UPDATE_TARGET", target)
	t.Setenv("SKILLPM_UPDATE_MANIFEST_URL", server.URL+"/manifest")

	svc := New(server.Client())
	res, err := svc.Update(context.Background(), "stable", true)
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}
	if !res.Updated || res.Version != "1.2.3" {
		t.Fatalf("unexpected result: %+v", res)
	}
	updated, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read updated binary failed: %v", err)
	}
	if string(updated) != string(newBin) {
		t.Fatalf("binary not updated")
	}
}

func TestUpdateChecksumMismatchFails(t *testing.T) {
	newBin := []byte("new-binary")
	target := filepath.Join(t.TempDir(), "skillpm")
	if err := os.WriteFile(target, []byte("old"), 0o755); err != nil {
		t.Fatalf("write target failed: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/bin", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(newBin)
	})
	mux.HandleFunc("/manifest", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(Manifest{Version: "1.0.0", URL: "/bin", Checksum: "deadbeef"})
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	t.Setenv("SKILLPM_SELF_UPDATE_TARGET", target)
	t.Setenv("SKILLPM_UPDATE_MANIFEST_URL", server.URL+"/manifest")

	svc := New(server.Client())
	if _, err := svc.Update(context.Background(), "stable", false); err == nil {
		t.Fatalf("expected checksum mismatch error")
	}
}

func TestUpdateRollbackOnInjectedSwapFailure(t *testing.T) {
	oldBin := []byte("old-binary")
	newBin := []byte("new-binary")
	target := filepath.Join(t.TempDir(), "skillpm")
	if err := os.WriteFile(target, oldBin, 0o755); err != nil {
		t.Fatalf("write target failed: %v", err)
	}
	h := sha256.Sum256(newBin)
	checksum := hex.EncodeToString(h[:])

	mux := http.NewServeMux()
	mux.HandleFunc("/bin", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(newBin)
	})
	mux.HandleFunc("/manifest", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(Manifest{Version: "1.0.0", URL: "/bin", Checksum: checksum})
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	t.Setenv("SKILLPM_SELF_UPDATE_TARGET", target)
	t.Setenv("SKILLPM_UPDATE_MANIFEST_URL", server.URL+"/manifest")
	t.Setenv("SKILLPM_TEST_FAIL_SELF_UPDATE_SWAP", "1")

	svc := New(server.Client())
	if _, err := svc.Update(context.Background(), "stable", false); err == nil {
		t.Fatalf("expected injected swap failure")
	}
	blob, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read target failed: %v", err)
	}
	if string(blob) != string(oldBin) {
		t.Fatalf("expected rollback to preserve previous binary")
	}
}

func TestResolveManifestURL(t *testing.T) {
	t.Setenv("SKILLPM_UPDATE_MANIFEST_BASE", "https://example.com/builds")
	
	// Test default base adjustment
	expectedDefault := "https://example.com/builds/manifest-stable-" + runtime.GOOS + "-" + runtime.GOARCH + ".json"
	if got := resolveManifestURL(""); got != expectedDefault {
		t.Errorf("resolveManifestURL(\"\") = %q; want %q", got, expectedDefault)
	}

	// Test custom channel
	expectedBeta := "https://example.com/builds/manifest-beta-" + runtime.GOOS + "-" + runtime.GOARCH + ".json"
	if got := resolveManifestURL("beta"); got != expectedBeta {
		t.Errorf("resolveManifestURL(\"beta\") = %q; want %q", got, expectedBeta)
	}

	// Test override
	t.Setenv("SKILLPM_UPDATE_MANIFEST_URL", "https://custom.com/manifest.json")
	if got := resolveManifestURL("beta"); got != "https://custom.com/manifest.json" {
		t.Errorf("resolveManifestURL override failed; got %q", got)
	}
}
