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

const (
	httpTimeout     = 90 * time.Second
	maxZipSizeBytes = 100 << 20 // 100 MiB
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

func unzipFile(file *zip.File, tempDir string) error {
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
	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return err
	}
	return nil
}

func DownloadAndUnzip(url string) (string, error) {
	zipReader, clean, err := downloadZipReader(url)
	if err != nil {
		return "", err
	}
	defer clean()

	tempDir, err := os.MkdirTemp("", "unzip-*")
	if err != nil {
		return "", err
	}
	for _, file := range zipReader.File {
		if zErr := unzipFile(file, tempDir); zErr != nil {
			return "", zErr
		}
	}
	return tempDir, nil
}
