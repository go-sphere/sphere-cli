package zip

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const httpTimeout = 90 * time.Second

const maxZipSizeBytes = 100 << 20 // 100 MiB download cap

// Extraction limits. Kept as variables so tests can shrink them.
var (
	maxZipEntries                    = 10_000
	maxUncompressedEntryBytes uint64 = 500 << 20 // 500 MiB per entry after decompression
	maxUncompressedTotalBytes uint64 = 1 << 30   // 1 GiB total after decompression
)

func downloadZipReader(url string) (*zip.Reader, func(), error) {
	client := http.Client{
		Timeout: httpTimeout,
	}
	resp, err := client.Get(url)
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK {
		return nil, nil, errors.New(resp.Status)
	}
	if resp.ContentLength > maxZipSizeBytes {
		return nil, nil, fmt.Errorf("zip file too large: %d bytes", resp.ContentLength)
	}
	tempFile, err := os.CreateTemp("", "zip-*")
	if err != nil {
		return nil, nil, err
	}
	cleanup := func() {
		_ = tempFile.Close()
		_ = os.Remove(tempFile.Name())
	}
	length, err := io.Copy(tempFile, io.LimitReader(resp.Body, maxZipSizeBytes+1))
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	if length > maxZipSizeBytes {
		cleanup()
		return nil, nil, fmt.Errorf("zip file too large: exceeded %d bytes", maxZipSizeBytes)
	}
	reader, err := zip.NewReader(tempFile, length)
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	return reader, func() {
		cleanup()
	}, nil
}

func ensureSafePath(tempDir, fileName string) (string, error) {
	basePath, err := filepath.Abs(filepath.Clean(tempDir))
	if err != nil {
		return "", err
	}
	filePath, err := filepath.Abs(filepath.Join(basePath, fileName))
	if err != nil {
		return "", err
	}
	relPath, err := filepath.Rel(basePath, filePath)
	if err != nil {
		return "", err
	}
	if relPath == ".." || strings.HasPrefix(relPath, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("unsafe file path: %s", filePath)
	}
	return filePath, nil
}

// countingWriter writes through to w and fails once total exceeds limit. It is
// used as a belt-and-braces cap on top of the zip header size checks.
type countingWriter struct {
	w     io.Writer
	n     int64
	limit int64
}

func (cw *countingWriter) Write(p []byte) (int, error) {
	if cw.n+int64(len(p)) > cw.limit {
		return 0, fmt.Errorf("uncompressed entry exceeds %d bytes", cw.limit)
	}
	n, err := cw.w.Write(p)
	cw.n += int64(n)
	return n, err
}

func unzipFile(file *zip.File, tempDir string, total *uint64) error {
	if file.FileInfo().Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("unsupported symlink entry: %s", file.Name)
	}
	// Pre-check the declared sizes from the zip header so oversized archives
	// are rejected before any data is decompressed.
	if file.UncompressedSize64 > maxUncompressedEntryBytes {
		return fmt.Errorf("uncompressed entry too large: %s (%d bytes)", file.Name, file.UncompressedSize64)
	}
	if *total+file.UncompressedSize64 > maxUncompressedTotalBytes {
		return fmt.Errorf("uncompressed archive too large: exceeds %d bytes", maxUncompressedTotalBytes)
	}

	filePath, err := ensureSafePath(tempDir, file.Name)
	if err != nil {
		return err
	}
	if file.FileInfo().IsDir() {
		return os.MkdirAll(filePath, 0o755)
	}
	err = os.MkdirAll(filepath.Dir(filePath), 0o755)
	if err != nil {
		return err
	}

	dstFile, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer func() {
		_ = dstFile.Close()
	}()

	srcFile, err := file.Open()
	if err != nil {
		return err
	}
	defer func() {
		_ = srcFile.Close()
	}()
	_, err = io.Copy(&countingWriter{w: dstFile, limit: int64(maxUncompressedEntryBytes)}, srcFile)
	if err != nil {
		return err
	}
	*total += file.UncompressedSize64
	return nil
}

func DownloadAndUnzip(url string) (dir string, err error) {
	zipReader, clean, err := downloadZipReader(url)
	if err != nil {
		return "", err
	}
	defer clean()

	tempDir, err := os.MkdirTemp("", "unzip-*")
	if err != nil {
		return "", err
	}
	defer func() {
		if err != nil {
			_ = os.RemoveAll(tempDir)
		}
	}()

	if len(zipReader.File) > maxZipEntries {
		return "", fmt.Errorf("archive has too many entries: %d", len(zipReader.File))
	}
	var totalUncompressed uint64
	for _, file := range zipReader.File {
		if err = unzipFile(file, tempDir, &totalUncompressed); err != nil {
			return "", err
		}
	}
	return tempDir, nil
}
