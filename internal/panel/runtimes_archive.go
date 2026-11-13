package panel

import (
	"archive/tar"
	"archive/zip"
	"compress/bzip2"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// 解压限额：便携运行时包体最大约 40MB，解压后通常在 200MB 以内。
const (
	archiveMaxEntries   = 60000
	archiveMaxFileBytes = 512 << 20
	archiveMaxTotal     = 2 << 30
)

// archiveLimits 汇总一次解压的累计状态。
type archiveLimits struct {
	entries int
	total   int64
}

// archiveTarget 把归档条目名映射到解压目标路径，并施加 stripRoot 规则。
// 返回空串表示该条目被 stripRoot 跳过（顶层目录本身）。
func (limits *archiveLimits) target(dest, name string, stripRoot bool) (string, error) {
	name = strings.TrimPrefix(filepath.ToSlash(name), "./")
	if name == "" || name == "." || name == "/" {
		return "", nil
	}
	limits.entries++
	if limits.entries > archiveMaxEntries {
		return "", errors.New("归档条目过多")
	}
	clean := strings.TrimSuffix(name, "/")
	if stripRoot {
		parts := strings.SplitN(clean, "/", 2)
		if len(parts) < 2 {
			// 顶层目录或顶层散文件：剥离后没有内容，跳过。
			return "", nil
		}
		clean = parts[1]
	}
	target, err := safePath(dest, clean)
	if err != nil {
		return "", fmt.Errorf("归档包含不安全路径 %q: %w", name, err)
	}
	return target, nil
}

func (limits *archiveLimits) account(size int64) error {
	if size < 0 || size > archiveMaxFileBytes {
		return errors.New("归档内单个文件过大")
	}
	limits.total += size
	if limits.total > archiveMaxTotal {
		return errors.New("归档解压后总大小超限")
	}
	return nil
}

// extractZip 安全解压 zip 到 dest，拒绝符号链接、硬链接与越界路径。
func extractZip(archive, dest string, stripRoot bool) error {
	reader, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer reader.Close()
	limits := &archiveLimits{}
	for _, entry := range reader.File {
		if entry.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("归档包含符号链接 %q，已拒绝", entry.Name)
		}
		target, err := limits.target(dest, entry.Name, stripRoot)
		if err != nil {
			return err
		}
		if target == "" {
			continue
		}
		if entry.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
			continue
		}
		if err := limits.account(int64(entry.UncompressedSize64)); err != nil {
			return err
		}
		source, err := entry.Open()
		if err != nil {
			return err
		}
		err = writeArchiveFile(target, source, entry.Mode().Perm())
		source.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

// extractTarBz2 安全解压 tar.bz2 到 dest；micromamba 等静态二进制使用此格式。
func extractTarBz2(archive, dest string, stripRoot bool) error {
	file, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer file.Close()
	return extractTarReader(tar.NewReader(bzip2.NewReader(file)), dest, stripRoot)
}

// extractTarGz 安全解压 tar.gz 到 dest，拒绝符号链接、硬链接与越界路径。
func extractTarGz(archive, dest string, stripRoot bool) error {
	file, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer file.Close()
	compressed, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer compressed.Close()
	return extractTarReader(tar.NewReader(compressed), dest, stripRoot)
}

// extractTarReader 是 tar.gz 与 tar.bz2 共用的安全解压实现。
func extractTarReader(reader *tar.Reader, dest string, stripRoot bool) error {
	limits := &archiveLimits{}
	for {
		header, err := reader.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		switch header.Typeflag {
		case tar.TypeSymlink, tar.TypeLink:
			return fmt.Errorf("归档包含链接 %q，已拒绝", header.Name)
		case tar.TypeChar, tar.TypeBlock, tar.TypeFifo:
			return fmt.Errorf("归档包含特殊文件 %q，已拒绝", header.Name)
		}
		target, err := limits.target(dest, header.Name, stripRoot)
		if err != nil {
			return err
		}
		if target == "" {
			continue
		}
		if header.Typeflag == tar.TypeDir {
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
			continue
		}
		if header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA {
			continue
		}
		if err := limits.account(header.Size); err != nil {
			return err
		}
		if err := writeArchiveFile(target, io.LimitReader(reader, archiveMaxFileBytes), os.FileMode(header.Mode).Perm()); err != nil {
			return err
		}
	}
}

// writeArchiveFile 先写临时文件再原子改名，避免解压中断留下半截文件。
func writeArchiveFile(target string, source io.Reader, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(target), ".extract-*")
	if err != nil {
		return err
	}
	temp := file.Name()
	defer os.Remove(temp)
	if _, err := io.Copy(file, source); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if mode != 0 {
		_ = os.Chmod(temp, mode)
	}
	return os.Rename(temp, target)
}

// extractRuntimeArchive 按扩展名分派。
func extractRuntimeArchive(archive, dest string, stripRoot bool) error {
	switch {
	case strings.HasSuffix(archive, ".zip"):
		return extractZip(archive, dest, stripRoot)
	case strings.HasSuffix(archive, ".tar.gz"), strings.HasSuffix(archive, ".tgz"):
		return extractTarGz(archive, dest, stripRoot)
	case strings.HasSuffix(archive, ".tar.bz2"), strings.HasSuffix(archive, ".tbz2"):
		return extractTarBz2(archive, dest, stripRoot)
	default:
		return fmt.Errorf("不支持的归档格式: %s", filepath.Base(archive))
	}
}
