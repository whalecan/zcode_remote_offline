package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestRealServerRelease(t *testing.T) {
	data, err := os.ReadFile("testdata/source-server-release.json")
	if err != nil {
		t.Fatal(err)
	}
	m, err := parseManifest(data, "3.16.3", "https://cdn.example/zcode/electron/releases")
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Components) != 1 {
		t.Fatalf("components=%v", m.Components)
	}
	c := m.Components[0]
	if c.ArtifactPath != "3.16.3/zcode-server-linux-x64.tar.gz" || c.Sha256 != "5062d871bd2ad2ddd1b44d6463bd9159af5b493be93845ffc7163dd5b5d45375" || !reflect.DeepEqual(c.URLs, []string{"https://cdn.example/zcode/electron/releases/3.16.3/zcode-server-linux-x64.tar.gz"}) {
		t.Fatalf("unexpected download plan: %+v", c)
	}
	if _, err := parseManifest(data, "3.16.1", "https://cdn.example/releases"); err == nil {
		t.Fatal("accepted wrong version")
	}
}

func TestServerReleaseDownload(t *testing.T) {
	data, err := os.ReadFile("testdata/source-server-release.json")
	if err != nil {
		t.Fatal(err)
	}
	var release serverRelease
	if err := json.Unmarshal(data, &release); err != nil {
		t.Fatal(err)
	}
	payload := []byte("simulated linux-x64 archive")
	hash := fmt.Sprintf("%x", sha256.Sum256(payload))
	release.Archives[0].Sha256 = hash
	// 首个镜像返回错误内容，校验失败后应自动尝试下一个。
	release.Archives[0].URLs = []string{"zcode-server-linux-x64.tar.gz", "mirror.tar.gz"}
	data, err = json.Marshal(release)
	if err != nil {
		t.Fatal(err)
	}
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/releases/3.16.3/source-server-release.json":
			w.Write(data)
		case "/releases/3.16.3/zcode-server-linux-x64.tar.gz":
			requests++
			w.Write([]byte("corrupt"))
		case "/releases/3.16.3/mirror.tar.gz":
			requests++
			w.Write(payload)
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	base := srv.URL + "/releases"
	resp, _, err := fetchManifest(srv.Client(), base, "3.16.3")
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	m, err := parseManifest(body, "3.16.3", base)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if len(m.Components) != 1 {
		t.Fatalf("components=%v", m.Components)
	}
	c := m.Components[0]
	for _, force := range []bool{false, false, true} {
		if err := ensureArtifact(srv.Client(), root, base, c, force); err != nil {
			t.Fatal(err)
		}
	}
	if requests != 4 {
		t.Fatalf("archive requests=%d, expected 4 (download, skip, force)", requests)
	}
	dest := filepath.Join(root, filepath.FromSlash(c.ArtifactPath))
	got, err := os.ReadFile(dest)
	if err != nil || string(got) != string(payload) {
		t.Fatalf("archive=%q error=%v", got, err)
	}
	if err := os.WriteFile(dest, []byte("damaged local file"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ensureArtifact(srv.Client(), root, base, c, false); err != nil {
		t.Fatal(err)
	}
	if requests != 6 {
		t.Fatalf("corrupt local file was not downloaded again: requests=%d", requests)
	}
	if err := writeSha256Sums(root, m); err != nil {
		t.Fatal(err)
	}
	sums, err := os.ReadFile(filepath.Join(root, "sha256sums-linux-x64.txt"))
	if err != nil || !strings.Contains(string(sums), hash+"  3.16.3/zcode-server-linux-x64.tar.gz") {
		t.Fatalf("sums=%s err=%v", sums, err)
	}
	// 在独立进程执行完整 main，检查原始清单和归档的落盘路径。
	workspace := t.TempDir()
	online := filepath.Join(workspace, "online")
	for _, dir := range []string{online, filepath.Join(workspace, "offline")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(online, "go.mod"), []byte("module smoke\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestCLIProcess$")
	cmd.Dir = online
	cmd.Env = append(os.Environ(), "ZCODE_TEST_CDN="+base)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("CLI failed: %v\n%s", err, output)
	}
	releases := filepath.Join(workspace, "offline", "zcode", "electron", "releases")
	for name, expected := range map[string][]byte{
		"3.16.3/source-server-release.json":    data,
		"3.16.3/zcode-server-linux-x64.tar.gz": payload,
	} {
		got, err := os.ReadFile(filepath.Join(releases, filepath.FromSlash(name)))
		if err != nil || string(got) != string(expected) {
			t.Fatalf("CLI file %s differs: err=%v", name, err)
		}
	}
}

func TestCLIProcess(t *testing.T) {
	base := os.Getenv("ZCODE_TEST_CDN")
	if base == "" {
		return
	}
	os.Args = []string{"download-offline-assets", "-version", "3.16.3", "-cdn", base}
	main()
	os.Exit(0)
}

func TestManifestValidation(t *testing.T) {
	for _, body := range []string{
		`{"version":"3.16.3","archives":[]}`,
		`{"version":"3.16.3","archives":[{"target":"linux-arm64"}]}`,
		`{"version":"3.16.3","archives":[{"target":"linux-x64","version":"3.16.1"}]}`,
		`{"appVersion":"3.16.3","platformArch":"linux-arm64","components":[{}]}`,
		`{"appVersion":"3.16.3","platformArch":"linux-x64","components":[]}`,
	} {
		if _, err := parseManifest([]byte(body), "3.16.3", "https://cdn.example/releases"); err == nil {
			t.Fatalf("accepted invalid manifest: %s", body)
		}
	}
	legacy := `{"appVersion":"3.4.2","platformArch":"linux-x64","components":[{"id":"node","artifactPath":"components/linux-x64/node/1.tar.gz"}]}`
	m, err := parseManifest([]byte(legacy), "3.4.2", "https://cdn.example/releases")
	if err != nil || len(m.Components) != 1 || m.Components[0].ArtifactPath != "components/linux-x64/node/1.tar.gz" {
		t.Fatalf("legacy manifest=%+v err=%v", m, err)
	}
}

func TestManifestCompatibility(t *testing.T) {
	for _, tc := range []struct {
		version string
		name    string
	}{
		{"3.4.2", "manifest-linux-x64.json"},
		{"3.16.1", "source-server-release.json"},
		{"3.16.3", "source-server-release.json"},
		{"4.0.0", "source-server-release.json"},
		// CDN 分批迁移时，优先名称可能不存在。
		{"3.16.3", "manifest-linux-x64.json"},
		{"3.15.0", "source-server-release.json"},
	} {
		t.Run(tc.version+"/"+tc.name, func(t *testing.T) {
			body := fmt.Sprintf(`{"appVersion":%q,"platformArch":"linux-x64","components":[{"id":"server-bundle"}]}`, tc.version)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/"+tc.version+"/"+tc.name {
					http.NotFound(w, r)
					return
				}
				fmt.Fprint(w, body)
			}))
			defer srv.Close()
			base, err := resolveBase(srv.Client(), tc.version, srv.URL)
			if err != nil {
				t.Fatal(err)
			}
			resp, name, err := fetchManifest(srv.Client(), base, tc.version)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			got, err := io.ReadAll(resp.Body)
			if err != nil || name != tc.name || string(got) != body {
				t.Fatalf("name=%q body=%q err=%v", name, got, err)
			}
			if got := probeRemoteVersions(srv.Client(), base, []string{tc.version, "0.0.0"}); !reflect.DeepEqual(got, []string{tc.version}) {
				t.Fatalf("remote versions=%v", got)
			}
		})
	}
}

func TestMissingManifestReportsBothNames(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	_, _, err := fetchManifest(srv.Client(), srv.URL, "3.16.3")
	if err == nil || !strings.Contains(err.Error(), "source-server-release.json: HTTP 404") || !strings.Contains(err.Error(), "manifest-linux-x64.json: HTTP 404") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLocalManifestCompatibility(t *testing.T) {
	root := t.TempDir()
	for version, name := range map[string]string{
		"3.4.2":  "manifest-linux-x64.json",
		"3.16.3": "source-server-release.json",
	} {
		dir := filepath.Join(root, version)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// 较新的空目录不应遮住已有清单的版本。
	if err := os.MkdirAll(filepath.Join(root, "3.17.0"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := detectDefaultVersion(root); got != "3.16.3" {
		t.Fatalf("default version=%q", got)
	}
	if got := listLocalVersions(root); !reflect.DeepEqual(got, map[string]bool{"3.4.2": true, "3.16.3": true}) {
		t.Fatalf("local versions=%v", got)
	}
}
