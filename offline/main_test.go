package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestServeHTTPServesEscapedPlusInArtifactPath(t *testing.T) {
	root := t.TempDir()
	artifact := filepath.Join(root, "components", "linux-x64", "zcode+remote", "1.2.3+build.tar.gz")
	if err := os.MkdirAll(filepath.Dir(artifact), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(artifact, []byte("artifact"), 0o644); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "http://cdn.test/components/linux-x64/zcode%2Bremote/1.2.3%2Bbuild.tar.gz", nil)
	res := httptest.NewRecorder()
	(&server{root: root}).ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", res.Code, http.StatusOK, res.Body.String())
	}
	if got, want := res.Body.String(), "artifact"; got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}
}
