package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestNewAndLegacyReleaseServing(t *testing.T) {
	root := t.TempDir()
	releases := filepath.Join(root, "zcode", "electron", "releases")
	write := func(rel, body string) {
		t.Helper()
		file := filepath.Join(releases, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("3.16.3/source-server-release.json", `{"version":"3.16.3","archives":[]}`)
	write("3.16.3/zcode-server-linux-x64.tar.gz", "archive")
	write("3.16.4/unrelated.json", "{}")
	if err := os.MkdirAll(filepath.Join(releases, "3.16.4", "source-server-release.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	expected := []versionInfo{{Version: "3.16.3", Manifests: []string{"source-server-release.json"}}}
	if got := listVersions(root); !reflect.DeepEqual(got, expected) {
		t.Fatalf("new-only versions=%v", got)
	}
	s := &server{root: root, bind: "127.0.0.1", port: "8765"}
	checkList := func() {
		t.Helper()
		w := httptest.NewRecorder()
		s.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/versions.json", nil))
		var response struct {
			Count    int           `json:"count"`
			Versions []versionInfo `json:"versions"`
		}
		if w.Code != 200 {
			t.Fatalf("list status=%d", w.Code)
		}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		if response.Count != len(expected) || !reflect.DeepEqual(response.Versions, expected) {
			t.Fatalf("versions response=%+v", response)
		}
	}
	checkList()
	write("3.4.2/manifest-linux-x64.json", "legacy")
	write("3.16.3/manifest-linux-x64.json", "coexisting")
	expected = []versionInfo{
		{Version: "3.16.3", Manifests: []string{"manifest-linux-x64.json", "source-server-release.json"}},
		{Version: "3.4.2", Manifests: []string{"manifest-linux-x64.json"}},
	}
	checkList() // 运行中新增旧版本和共存清单后，应立即重新扫描。
	for rel, body := range map[string]string{
		"3.16.3/source-server-release.json":    `{"version":"3.16.3","archives":[]}`,
		"3.16.3/zcode-server-linux-x64.tar.gz": "archive",
		"3.4.2/manifest-linux-x64.json":        "legacy",
	} {
		for _, method := range []string{http.MethodGet, http.MethodHead} {
			w := httptest.NewRecorder()
			s.ServeHTTP(w, httptest.NewRequest(method, "/zcode/electron/releases/"+rel, nil))
			if w.Code != 200 {
				t.Fatalf("%s %s: status=%d", method, rel, w.Code)
			}
			if method == http.MethodGet && w.Body.String() != body {
				t.Fatalf("%s: incorrect body", rel)
			}
			if method == http.MethodHead && w.Body.Len() != 0 {
				t.Fatal("HEAD returned a body")
			}
		}
	}
	w := httptest.NewRecorder()
	s.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	if !strings.Contains(w.Body.String(), "/3.16.3/source-server-release.json") {
		t.Fatal("new manifest missing from homepage")
	}
}
