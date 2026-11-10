package panel

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type fixtureTransport func(*http.Request) (*http.Response, error)

func (transport fixtureTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport(request)
}

func TestSignedUpdateRejectsTampering(test *testing.T) {
	panel := setupPanel(test)
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		test.Fatal(err)
	}
	panel.app.updateKey = base64.StdEncoding.EncodeToString(public)
	if err := panel.app.store.setSetting("githubRepo", "example/cabbage"); err != nil {
		test.Fatal(err)
	}
	binary := []byte("fixture executable bytes, never executed")
	digest := sha256.Sum256(binary)
	manifest, err := json.Marshal(UpdateManifest{Version: "1.1.0", Assets: []UpdateAsset{{OS: runtime.GOOS, Arch: runtime.GOARCH, Name: "cabbage-fixture", SHA256: hex.EncodeToString(digest[:])}}})
	if err != nil {
		test.Fatal(err)
	}
	signature := base64.StdEncoding.EncodeToString(ed25519.Sign(private, manifest))
	release := Release{Tag: "v1.1.0", Assets: []ReleaseAsset{{Name: "manifest.json", URL: "https://github.com/example/cabbage/manifest.json"}, {Name: "manifest.sig", URL: "https://github.com/example/cabbage/manifest.sig"}, {Name: "cabbage-fixture", URL: "https://github.com/example/cabbage/cabbage-fixture"}}}
	releaseBytes, _ := json.Marshal(release)
	responses := map[string][]byte{"/repos/example/cabbage/releases/latest": releaseBytes, "/example/cabbage/manifest.json": manifest, "/example/cabbage/manifest.sig": []byte(signature), "/example/cabbage/cabbage-fixture": binary}
	panel.app.client.Transport = fixtureTransport(func(request *http.Request) (*http.Response, error) {
		data, found := responses[request.URL.Path]
		status := 200
		if !found {
			status = 404
		}
		return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(data)), Request: request}, nil
	})
	if err := panel.app.stageUpdate(context.Background(), io.Discard); err != nil {
		test.Fatal(err)
	}
	staged, err := os.ReadFile(filepath.Join(panel.app.root, "updates", "cabbage.next"))
	if err != nil || !bytes.Equal(staged, binary) {
		test.Fatal("staged bytes mismatch", err)
	}
	responses["/example/cabbage/cabbage-fixture"] = []byte("tampered")
	if err := panel.app.stageUpdate(context.Background(), io.Discard); err == nil || !strings.Contains(err.Error(), "SHA-256") {
		test.Fatal("tampered executable accepted", err)
	}
	responses["/example/cabbage/manifest.json"] = append(manifest, ' ')
	if err := panel.app.stageUpdate(context.Background(), io.Discard); err == nil || !strings.Contains(err.Error(), "签名") {
		test.Fatal("tampered manifest accepted", err)
	}
}

func TestDataDirectoryExclusiveLock(test *testing.T) {
	panel := setupPanel(test)
	second, err := New(panel.app.root, "1.0.0", "test", "")
	if err == nil {
		second.Close()
		test.Fatal("second process opened the same data directory")
	}
	if !strings.Contains(err.Error(), "占用") {
		test.Fatal(err)
	}
}

func TestBackupBusyReturnsImmediately(test *testing.T) {
	panel := setupPanel(test)
	id := panel.task(test, "active", "sleep 20", nil)
	runID := panel.run(test, id)
	status, _ := panel.call(test, "POST", "/api/backups/export", nil, panel.token)
	if status != 409 {
		test.Fatal("backup did not report busy", status)
	}
	panel.data(test, "PUT", "/api/crons/stop", []int64{id})
	if panel.finished(test, runID)["status"] != "cancelled" {
		test.Fatal("cancel after backup failed")
	}
}
