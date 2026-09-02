// Package update provides self-update support from GitHub Releases.
package update

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const latestReleaseURL = "https://api.github.com/repos/PubbleUp/codex-session-manager/releases/latest"

var defaultClient = &http.Client{Timeout: 30 * time.Second}

type release struct {
	TagName string  `json:"tag_name"`
	Draft   bool    `json:"draft"`
	Assets  []asset `json:"assets"`
}

type asset struct {
	Name   string `json:"name"`
	URL    string `json:"browser_download_url"`
	Digest string `json:"digest"`
}

// Result describes the result of an update check.
type Result struct {
	Updated bool
	Current string
	Latest  string
	Asset   string
}

// Update checks the latest stable GitHub Release and replaces the running binary
// when a newer release for the current platform is available.
func Update(current string) (Result, error) {
	return updateFromURL(defaultClient, latestReleaseURL, current, os.Executable, os.Rename)
}

func updateFromURL(client *http.Client, endpoint, current string, executable func() (string, error), rename func(string, string) error) (Result, error) {
	if client == nil {
		client = http.DefaultClient
	}
	request, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return Result{}, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "codex-session-manager")
	response, err := client.Do(request)
	if err != nil {
		return Result{}, fmt.Errorf("检查 GitHub 最新版本失败：%w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return Result{}, fmt.Errorf("检查 GitHub 最新版本失败：HTTP %s", response.Status)
	}
	var latest release
	if err := json.NewDecoder(response.Body).Decode(&latest); err != nil {
		return Result{}, fmt.Errorf("解析 GitHub Release 失败：%w", err)
	}
	result := Result{Current: current, Latest: latest.TagName}
	if latest.Draft || !isNewer(latest.TagName, current) {
		return result, nil
	}

	name := fmt.Sprintf("codex-session-manager_%s_%s_%s", strings.TrimPrefix(latest.TagName, "v"), runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	var selected *asset
	for i := range latest.Assets {
		if latest.Assets[i].Name == name {
			selected = &latest.Assets[i]
			break
		}
	}
	if selected == nil {
		return result, fmt.Errorf("Release %s 没有当前平台资产：%s", latest.TagName, name)
	}
	if selected.Digest == "" || !strings.HasPrefix(selected.Digest, "sha256:") {
		return result, fmt.Errorf("Release 资产缺少 SHA-256 摘要：%s", name)
	}

	oldPath, err := executable()
	if err != nil {
		return result, fmt.Errorf("定位当前可执行文件失败：%w", err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(oldPath), ".codex-session-manager-update-*")
	if err != nil {
		return result, fmt.Errorf("创建更新临时文件失败：%w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := download(client, selected.URL, temporary); err != nil {
		temporary.Close()
		return result, err
	}
	if err := temporary.Close(); err != nil {
		return result, fmt.Errorf("关闭更新临时文件失败：%w", err)
	}
	if err := verifyDigest(temporaryPath, strings.TrimPrefix(selected.Digest, "sha256:")); err != nil {
		return result, err
	}
	mode := os.FileMode(0755)
	if info, err := os.Stat(oldPath); err == nil {
		mode = info.Mode()
	}
	if err := os.Chmod(temporaryPath, mode); err != nil {
		return result, fmt.Errorf("设置更新文件权限失败：%w", err)
	}
	if err := rename(temporaryPath, oldPath); err != nil {
		return result, fmt.Errorf("替换当前可执行文件失败：%w", err)
	}
	result.Updated = true
	result.Asset = name
	return result, nil
}

func download(client *http.Client, url string, destination io.Writer) error {
	request, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("创建更新下载请求失败：%w", err)
	}
	request.Header.Set("User-Agent", "codex-session-manager")
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("下载更新失败：%w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("下载更新失败：HTTP %s", response.Status)
	}
	if _, err := io.Copy(destination, response.Body); err != nil {
		return fmt.Errorf("写入更新文件失败：%w", err)
	}
	return nil
}

func verifyDigest(path, expected string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("读取更新文件失败：%w", err)
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return fmt.Errorf("计算更新文件摘要失败：%w", err)
	}
	if hex.EncodeToString(hash.Sum(nil)) != strings.ToLower(expected) {
		return fmt.Errorf("更新文件 SHA-256 校验失败")
	}
	return nil
}

func isNewer(latest, current string) bool {
	latestParts, latestOK := versionParts(latest)
	currentParts, currentOK := versionParts(current)
	if !latestOK || !currentOK {
		return strings.TrimPrefix(latest, "v") != strings.TrimPrefix(current, "v")
	}
	for i := range latestParts {
		if latestParts[i] != currentParts[i] {
			return latestParts[i] > currentParts[i]
		}
	}
	return false
}

func versionParts(value string) ([3]int, bool) {
	var parts [3]int
	value = strings.TrimPrefix(strings.TrimSpace(value), "v")
	var suffix string
	if index := strings.IndexByte(value, '-'); index >= 0 {
		suffix = value[index:]
		value = value[:index]
	}
	if suffix != "" {
		return parts, false
	}
	var count int
	if _, err := fmt.Sscanf(value, "%d.%d.%d", &parts[0], &parts[1], &parts[2]); err != nil {
		return parts, false
	}
	for _, character := range value {
		if character == '.' {
			count++
		}
	}
	return parts, count == 2
}
