package panel

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// makeZip 构造内存 zip；entries 键为条目名，值为内容。
func makeZip(test *testing.T, entries map[string]string) []byte {
	test.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, content := range entries {
		file, err := writer.Create(name)
		if err != nil {
			test.Fatal(err)
		}
		if _, err := file.Write([]byte(content)); err != nil {
			test.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		test.Fatal(err)
	}
	return buffer.Bytes()
}

func makeTarGz(test *testing.T, entries map[string]string) []byte {
	test.Helper()
	var buffer bytes.Buffer
	compressed := gzip.NewWriter(&buffer)
	writer := tar.NewWriter(compressed)
	for name, content := range entries {
		header := &tar.Header{Name: name, Mode: 0644, Size: int64(len(content)), Typeflag: tar.TypeReg}
		if err := writer.WriteHeader(header); err != nil {
			test.Fatal(err)
		}
		if _, err := writer.Write([]byte(content)); err != nil {
			test.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		test.Fatal(err)
	}
	if err := compressed.Close(); err != nil {
		test.Fatal(err)
	}
	return buffer.Bytes()
}

func TestExtractZipStripsRootAndWritesFiles(t *testing.T) {
	directory := t.TempDir()
	archive := filepath.Join(directory, "python.zip")
	if err := os.WriteFile(archive, makeZip(t, map[string]string{
		"python/bin/python3": "#!/bin/sh\n",
		"python/lib/README":  "hi",
	}), 0600); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(directory, "out")
	if err := extractZip(archive, dest, true); err != nil {
		t.Fatalf("extractZip: %v", err)
	}
	content, err := os.ReadFile(filepath.Join(dest, "bin", "python3"))
	if err != nil {
		t.Fatalf("缺少解压后的文件: %v", err)
	}
	if string(content) != "#!/bin/sh\n" {
		t.Fatalf("文件内容不符: %q", content)
	}
}

func TestExtractZipRejectsTraversal(t *testing.T) {
	directory := t.TempDir()
	archive := filepath.Join(directory, "evil.zip")
	if err := os.WriteFile(archive, makeZip(t, map[string]string{
		"pkg/../../escape.txt": "boom",
	}), 0600); err != nil {
		t.Fatal(err)
	}
	err := extractZip(archive, filepath.Join(directory, "out"), true)
	if err == nil {
		t.Fatal("路径穿越应被拒绝")
	}
	if _, statErr := os.Stat(filepath.Join(directory, "escape.txt")); statErr == nil {
		t.Fatal("越界文件不应被写出")
	}
}

func TestExtractZipRejectsSymlink(t *testing.T) {
	directory := t.TempDir()
	archive := filepath.Join(directory, "link.zip")
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	header := &zip.FileHeader{Name: "pkg/link"}
	header.SetMode(os.ModeSymlink | 0777)
	file, err := writer.CreateHeader(header)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = file.Write([]byte("/etc/passwd"))
	_ = writer.Close()
	if err := os.WriteFile(archive, buffer.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	if err := extractZip(archive, filepath.Join(directory, "out"), true); err == nil {
		t.Fatal("符号链接应被拒绝")
	}
}

func TestExtractTarGzRejectsHardLink(t *testing.T) {
	directory := t.TempDir()
	archive := filepath.Join(directory, "hard.tar.gz")
	var buffer bytes.Buffer
	compressed := gzip.NewWriter(&buffer)
	writer := tar.NewWriter(compressed)
	header := &tar.Header{Name: "pkg/link", Typeflag: tar.TypeLink, Linkname: "/etc/passwd"}
	if err := writer.WriteHeader(header); err != nil {
		t.Fatal(err)
	}
	_ = writer.Close()
	_ = compressed.Close()
	if err := os.WriteFile(archive, buffer.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	if err := extractTarGz(archive, filepath.Join(directory, "out"), true); err == nil {
		t.Fatal("硬链接应被拒绝")
	}
}

func TestExtractTarGzWithoutStripRoot(t *testing.T) {
	directory := t.TempDir()
	archive := filepath.Join(directory, "git.tar.gz")
	if err := os.WriteFile(archive, makeTarGz(t, map[string]string{
		"cmd/git.exe": "fake",
	}), 0600); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(directory, "out")
	if err := extractTarGz(archive, dest, false); err != nil {
		t.Fatalf("extractTarGz: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "cmd", "git.exe")); err != nil {
		t.Fatalf("未剥离时应保留顶层目录: %v", err)
	}
}

// makeTarBz2 构造内存 tar.bz2；标准库无 bzip2 写支持，故预置一段固定压缩数据不可行，
// 这里改为跳过构造、直接用真实 micromamba 包做集成断言（见 TestExtractTarBz2RejectsTraversal）。
func TestExtractTarBz2RejectsTraversal(t *testing.T) {
	// 用系统 bzip2 命令构造最小包；若环境没有则跳过，不伪造通过。
	bzip2Path, err := exec.LookPath("bzip2")
	if err != nil {
		t.Skip("未找到 bzip2 命令，跳过 tar.bz2 构造测试")
	}
	directory := t.TempDir()
	tarPath := filepath.Join(directory, "evil.tar")
	archive := filepath.Join(directory, "evil.tar.bz2")
	// 手写一个含 ../ 路径的 tar。
	var buffer bytes.Buffer
	writer := tar.NewWriter(&buffer)
	header := &tar.Header{Name: "pkg/../../escape.txt", Mode: 0644, Size: 4, Typeflag: tar.TypeReg}
	if err := writer.WriteHeader(header); err != nil {
		t.Fatal(err)
	}
	_, _ = writer.Write([]byte("boom"))
	_ = writer.Close()
	if err := os.WriteFile(tarPath, buffer.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(bzip2Path, "-c", tarPath)
	compressed, err := command.Output()
	if err != nil {
		t.Skipf("bzip2 压缩失败: %v", err)
	}
	if err := os.WriteFile(archive, compressed, 0600); err != nil {
		t.Fatal(err)
	}
	if err := extractTarBz2(archive, filepath.Join(directory, "out"), true); err == nil {
		t.Fatal("tar.bz2 路径穿越应被拒绝")
	}
	if _, statErr := os.Stat(filepath.Join(directory, "escape.txt")); statErr == nil {
		t.Fatal("越界文件不应被写出")
	}
}

func TestVerifyFileSHA256(t *testing.T) {
	path := filepath.Join(t.TempDir(), "payload")
	if err := os.WriteFile(path, []byte("cabbage"), 0600); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte("cabbage"))
	if err := verifyFileSHA256(path, hex.EncodeToString(digest[:])); err != nil {
		t.Fatalf("正确指纹应通过: %v", err)
	}
	if err := verifyFileSHA256(path, strings.Repeat("0", 64)); err == nil {
		t.Fatal("错误指纹应被拒绝")
	}
	if err := verifyFileSHA256(path, ""); err == nil {
		t.Fatal("缺少指纹应被拒绝")
	}
}

// TestInstallPortableRuntime 用 httptest 提供假包，验证下载 → 校验 → 解压 → 替换全流程。
func TestInstallPortableRuntime(t *testing.T) {
	panel := setupPanel(t)
	name := "Python"
	inner := map[string]string{"bin/python3": "#!/bin/sh\necho 3.12.16\n"}
	if runtime.GOOS == "windows" {
		inner = map[string]string{"python.exe": "fake"}
	}
	payload := makeTarGz(t, prefixEntries("python", inner))
	digest := sha256.Sum256(payload)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Length", strconv.Itoa(len(payload)))
		_, _ = writer.Write(payload)
	}))
	defer server.Close()

	asset := &runtimeAsset{
		Runtime:     name,
		Version:     "test",
		GOOS:        runtime.GOOS,
		GOARCH:      runtime.GOARCH,
		File:        "python-test.tar.gz",
		DefaultBase: server.URL,
		MirrorKey:   "runtimePythonMirror",
		SHA256:      hex.EncodeToString(digest[:]),
		Size:        int64(len(payload)),
		StripRoot:   true,
	}
	if err := panel.app.installPortableRuntime(context.Background(), asset); err != nil {
		t.Fatalf("installPortableRuntime: %v", err)
	}
	if portable := panel.app.portablePython(); portable == "" {
		t.Fatal("安装后应探测到便携 Python")
	}
	progress := panel.app.currentRuntimeProgress()
	if progress.Received == 0 {
		t.Fatal("下载进度未上报")
	}
}

func TestInstallPortableRuntimeRejectsBadDigest(t *testing.T) {
	panel := setupPanel(t)
	inner := map[string]string{"bin/python3": "#!/bin/sh\n"}
	if runtime.GOOS == "windows" {
		inner = map[string]string{"python.exe": "fake"}
	}
	payload := makeTarGz(t, prefixEntries("python", inner))
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write(payload)
	}))
	defer server.Close()

	asset := &runtimeAsset{
		Runtime:     "Python",
		GOOS:        runtime.GOOS,
		GOARCH:      runtime.GOARCH,
		File:        "python-test.tar.gz",
		DefaultBase: server.URL,
		MirrorKey:   "runtimePythonMirror",
		SHA256:      strings.Repeat("a", 64),
		StripRoot:   true,
	}
	err := panel.app.installPortableRuntime(context.Background(), asset)
	if err == nil {
		t.Fatal("指纹不匹配应失败")
	}
	if panel.app.portablePython() != "" {
		t.Fatal("校验失败不应留下运行时目录")
	}
	// 临时目录必须被清理。
	matches, _ := filepath.Glob(filepath.Join(panel.app.root, "packages", "runtime", ".tmp-*"))
	if len(matches) != 0 {
		t.Fatalf("临时目录未清理: %v", matches)
	}
}

func TestRuntimeEntriesShape(t *testing.T) {
	panel := setupPanel(t)
	result := panel.data(t, "GET", "/api/system/runtimes", nil).([]any)
	seen := map[string]bool{}
	for _, item := range result {
		entry := item.(map[string]any)
		name := text(entry["name"])
		seen[name] = true
		if _, ok := entry["installable"]; !ok {
			t.Fatalf("%s 缺少 installable 字段", name)
		}
		if _, ok := entry["source"]; !ok {
			t.Fatalf("%s 缺少 source 字段", name)
		}
	}
	for _, name := range runtimeNames {
		if !seen[name] {
			t.Fatalf("缺少运行时 %s", name)
		}
	}
}

func TestRuntimeInstallRejectsUnknownName(t *testing.T) {
	panel := setupPanel(t)
	status, _ := panel.call(t, "POST", "/api/system/runtimes/install", Object{"name": "Ruby"}, panel.token)
	if status != 400 {
		t.Fatalf("未知运行时应返回 400，实际 %d", status)
	}
}

func TestRuntimeProgressEndpoint(t *testing.T) {
	panel := setupPanel(t)
	result := panel.data(t, "GET", "/api/system/runtimes/progress", nil).(map[string]any)
	if boolean(result["running"]) {
		t.Fatal("初始状态不应在运行")
	}
}

// prefixEntries 给所有条目加统一顶层目录，模拟 install_only 包的布局。
func prefixEntries(prefix string, entries map[string]string) map[string]string {
	result := make(map[string]string, len(entries))
	for name, content := range entries {
		result[prefix+"/"+name] = content
	}
	return result
}
