// 从公网 CDN 下载 ZCode 远程资源到同级 ../offline 目录（仅 linux-x64）。
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

const platformArch = "linux-x64"

var (
	defaultCDNCandidates = []string{
		"https://cdn-zcode.z.ai/zcode/electron/releases",
		"https://cdn.codegeex.cn/zcode/electron/releases",
		"https://cdn.zcode-ai.com/zcode/electron/releases",
	}
	versionDirRE = regexp.MustCompile(`^\d+\.\d+\.\d+`)
)

type component struct {
	ID           string `json:"id"`
	Version      string `json:"version"`
	Sha256       string `json:"sha256"`
	ArtifactPath string `json:"artifactPath"`
}

type manifest struct {
	SchemaVersion int         `json:"schemaVersion"`
	AppVersion    string      `json:"appVersion"`
	PlatformArch  string      `json:"platformArch"`
	Components    []component `json:"components"`
}

func trimSlash(s string) string {
	return strings.TrimRight(s, "/")
}

func encodePath(p string) string {
	parts := strings.Split(p, "/")
	for i, seg := range parts {
		parts[i] = url.PathEscape(seg)
	}
	return strings.Join(parts, "/")
}

func parseVersionParts(v string) []int {
	base := v
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		base = v[:i]
	}
	parts := strings.Split(base, ".")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			out = append(out, 0)
			continue
		}
		out = append(out, n)
	}
	return out
}

func versionLess(a, b string) bool {
	pa, pb := parseVersionParts(a), parseVersionParts(b)
	n := len(pa)
	if len(pb) > n {
		n = len(pb)
	}
	for i := 0; i < n; i++ {
		var x, y int
		if i < len(pa) {
			x = pa[i]
		}
		if i < len(pb) {
			y = pb[i]
		}
		if x != y {
			return x < y
		}
	}
	return a < b
}

func detectDefaultVersion(releasesRoot string) string {
	entries, err := os.ReadDir(releasesRoot)
	if err != nil {
		return ""
	}
	var versions []string
	for _, e := range entries {
		if e.IsDir() && versionDirRE.MatchString(e.Name()) {
			versions = append(versions, e.Name())
		}
	}
	sort.Slice(versions, func(i, j int) bool {
		return versionLess(versions[j], versions[i])
	})
	for _, ver := range versions {
		mp := filepath.Join(releasesRoot, ver, "manifest-"+platformArch+".json")
		if _, err := os.Stat(mp); err == nil {
			return ver
		}
	}
	if len(versions) > 0 {
		return versions[0]
	}
	return ""
}

func httpClient() *http.Client {
	return &http.Client{
		Timeout: 30 * time.Minute,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}
}

func fetchOK(client *http.Client, rawURL string) (*http.Response, error) {
	resp, err := client.Get(rawURL)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return resp, nil
}

func resolveBase(client *http.Client, version, preferred string) (string, error) {
	if preferred != "" {
		preferred = trimSlash(preferred)
		u := preferred + "/" + version + "/manifest-" + platformArch + ".json"
		resp, err := fetchOK(client, u)
		if err != nil {
			return "", fmt.Errorf("指定 CDN 不可用：%w", err)
		}
		_ = resp.Body.Close()
		fmt.Println("CDN 可用（指定）：", preferred)
		return preferred, nil
	}
	var lastErr error
	for _, base := range defaultCDNCandidates {
		u := base + "/" + version + "/manifest-" + platformArch + ".json"
		resp, err := fetchOK(client, u)
		if err != nil {
			fmt.Println("CDN 不可用：", base, err.Error())
			lastErr = err
			continue
		}
		_ = resp.Body.Close()
		fmt.Println("CDN 可用：", base)
		return base, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("无候选")
	}
	return "", fmt.Errorf("没有可用的 CDN。请使用 -cdn <地址> 指定，或检查网络：%w", lastErr)
}

func downloadTo(client *http.Client, rawURL, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	tmp := dest + ".partial"
	resp, err := fetchOK(client, rawURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	total := resp.ContentLength
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}

	var done int64
	lastLog := time.Time{}
	buf := make([]byte, 256*1024)
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := out.Write(buf[:n]); werr != nil {
				_ = out.Close()
				_ = os.Remove(tmp)
				return werr
			}
			done += int64(n)
			now := time.Now()
			if now.Sub(lastLog) > 800*time.Millisecond {
				lastLog = now
				if total > 0 {
					fmt.Printf("\r  %.1f%% (%.1f/%.1f MB)", float64(done)/float64(total)*100, float64(done)/1048576, float64(total)/1048576)
				} else {
					fmt.Printf("\r  %.1f MB", float64(done)/1048576)
				}
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			_ = out.Close()
			_ = os.Remove(tmp)
			return rerr
		}
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	fmt.Printf("\r  100%% (%.1f MB)\n", float64(done)/1048576)
	_ = os.Remove(dest)
	return os.Rename(tmp, dest)
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func ensureArtifact(client *http.Client, releasesRoot, cdnBase string, c component, force bool) error {
	dest := filepath.Join(append([]string{releasesRoot}, strings.Split(c.ArtifactPath, "/")...)...)
	expected := strings.TrimSpace(c.Sha256)

	if !force {
		if st, err := os.Stat(dest); err == nil && !st.IsDir() {
			if expected != "" {
				actual, err := sha256File(dest)
				if err != nil {
					return err
				}
				if actual == expected {
					fmt.Printf("[跳过] %s sha256 校验通过（%.1f MB）\n", c.ID, float64(st.Size())/1048576)
					return nil
				}
				fmt.Printf("[重新下载] %s sha256 不匹配 期望=%s 实际=%s\n", c.ID, expected, actual)
			} else {
				fmt.Printf("[跳过] %s 本地已存在\n", c.ID)
				return nil
			}
		}
	}

	rawURL := cdnBase + "/" + encodePath(c.ArtifactPath)
	forceTag := ""
	if force {
		forceTag = "（强制）"
	}
	fmt.Printf("[下载] %s%s\n", c.ID, forceTag)
	fmt.Println(" ", rawURL)
	fmt.Println("  ->", dest)
	if err := downloadTo(client, rawURL, dest); err != nil {
		return err
	}
	if expected != "" {
		actual, err := sha256File(dest)
		if err != nil {
			return err
		}
		if actual != expected {
			return fmt.Errorf("%s 的 sha256 不匹配：期望=%s 实际=%s", c.ID, expected, actual)
		}
		fmt.Println("  sha256 校验通过")
	}
	return nil
}

func writeSha256Sums(offlineRoot string, m manifest) error {
	var b strings.Builder
	b.WriteString("# ZCode ")
	b.WriteString(m.AppVersion)
	b.WriteString(" / ")
	b.WriteString(platformArch)
	b.WriteString(" 组件压缩包 SHA256\n")
	b.WriteString("# 与 manifest.components[].sha256 一致\n\n")
	for _, c := range m.Components {
		b.WriteString(c.Sha256)
		b.WriteString("  ")
		b.WriteString(c.ArtifactPath)
		b.WriteByte('\n')
	}
	b.WriteByte('\n')
	out := filepath.Join(offlineRoot, "sha256sums-"+platformArch+".txt")
	if err := os.WriteFile(out, []byte(b.String()), 0o644); err != nil {
		return err
	}
	fmt.Println("[写入]", out)
	return nil
}

func printInventory(root string) {
	fmt.Println("\n=== 离线资源清单 ===")
	var walk func(dir, prefix string)
	walk = func(dir, prefix string) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		for _, e := range entries {
			p := filepath.Join(dir, e.Name())
			if e.IsDir() {
				walk(p, prefix+e.Name()+"/")
				continue
			}
			st, err := e.Info()
			if err != nil {
				continue
			}
			fmt.Printf("%8.2f MB  %s%s\n", float64(st.Size())/1048576, prefix, e.Name())
		}
	}
	walk(root, "")
}

func printHelp() {
	fmt.Print(`下载 ZCode 远程 SSH 离线 CDN 资源（仅 linux-x64）。

本工具位于 online/，下载结果写入同级 offline/ 目录。

用法：
  go run . [选项]
  go build -o download-offline-assets.exe .
  ./download-offline-assets.exe [选项]

选项：
  -version, -v <版本号>   要拉取的应用版本（默认：offline 中已有的最高版本）
  -cdn <地址>             强制指定 CDN releases 根地址（跳过自动探测）
  -force, -f              即使本地 sha256 已匹配也强制重新下载
  -help, -h               显示帮助

示例：
  go run . -version 3.4.2
  go run . -version 3.5.0 -force
  go run . -version 3.4.2 -cdn https://cdn-zcode.z.ai/zcode/electron/releases
`)
}

func findOnlineRoot() string {
	var candidates []string
	if wd, err := os.Getwd(); err == nil {
		candidates = append(candidates, wd)
	}
	if exe, err := os.Executable(); err == nil {
		if resolved, err := filepath.EvalSymlinks(exe); err == nil {
			candidates = append(candidates, filepath.Dir(resolved))
		}
	}
	if _, file, _, ok := runtime.Caller(0); ok {
		candidates = append(candidates, filepath.Dir(file))
	}
	for _, c := range candidates {
		if c == "" {
			continue
		}
		if _, err := os.Stat(filepath.Join(c, "go.mod")); err != nil {
			continue
		}
		if _, err := os.Stat(filepath.Join(c, "..", "offline")); err == nil {
			return c
		}
	}
	if wd, err := os.Getwd(); err == nil {
		return wd
	}
	return "."
}

func main() {
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	var (
		version string
		cdn     string
		force   bool
		help    bool
	)
	fs.StringVar(&version, "version", "", "应用版本")
	fs.StringVar(&version, "v", "", "应用版本（简写）")
	fs.StringVar(&cdn, "cdn", "", "CDN releases 根地址")
	fs.BoolVar(&force, "force", false, "强制重新下载")
	fs.BoolVar(&force, "f", false, "强制重新下载（简写）")
	fs.BoolVar(&help, "help", false, "帮助")
	fs.BoolVar(&help, "h", false, "帮助（简写）")
	_ = fs.Parse(os.Args[1:])

	if help {
		printHelp()
		return
	}

	onlineRoot := findOnlineRoot()
	offlineRoot := filepath.Clean(filepath.Join(onlineRoot, "..", "offline"))
	releasesRoot := filepath.Join(offlineRoot, "zcode", "electron", "releases")

	if version == "" {
		version = detectDefaultVersion(releasesRoot)
	}
	if version == "" {
		fatal(fmt.Errorf("未指定版本，且 offline 中也没有可用的清单文件。请使用 -version <版本号>。"))
	}

	fmt.Println("在线目录     :", onlineRoot)
	fmt.Println("离线目录     :", offlineRoot)
	fmt.Println("版本         :", version)
	fmt.Println("目标平台     :", platformArch)
	fmt.Println("强制下载     :", force)

	client := httpClient()
	cdnBase, err := resolveBase(client, version, cdn)
	if err != nil {
		fatal(err)
	}

	manifestPath := filepath.Join(releasesRoot, version, "manifest-"+platformArch+".json")
	manifestURL := cdnBase + "/" + version + "/manifest-" + platformArch + ".json"
	fmt.Println("[下载] 清单", manifestURL)

	resp, err := fetchOK(client, manifestURL)
	if err != nil {
		fatal(err)
	}
	mText, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil {
		fatal(err)
	}
	if err := os.WriteFile(manifestPath, mText, 0o644); err != nil {
		fatal(err)
	}

	var fresh manifest
	if err := json.Unmarshal(mText, &fresh); err != nil {
		fatal(err)
	}
	if fresh.PlatformArch != platformArch {
		fatal(fmt.Errorf("platformArch 不匹配：实际=%s 期望=%s", fresh.PlatformArch, platformArch))
	}
	if fresh.AppVersion != version {
		fatal(fmt.Errorf("appVersion 不匹配：实际=%s 期望=%s", fresh.AppVersion, version))
	}

	for _, c := range fresh.Components {
		if err := ensureArtifact(client, releasesRoot, cdnBase, c, force); err != nil {
			fatal(err)
		}
	}

	if err := writeSha256Sums(offlineRoot, fresh); err != nil {
		fatal(err)
	}
	printInventory(filepath.Join(releasesRoot, version))
	printInventory(filepath.Join(releasesRoot, "components", platformArch))
	fmt.Println("完成")
	fmt.Printf("\n资源已写入：%s\n下一步（在离线机器上）：cd offline && go run . ，并设置环境变量 ZCODE_REMOTE_ASSET_CDN_BASE_URL。\n", offlineRoot)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err.Error())
	os.Exit(1)
}
