package upload

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
)

const maxExtractionSize = 256 << 20

type UploadHandler struct {
	userCodePath       string
	userCodeEntrypoint string
	onUpload           func()
}

func NewHandler(userCodePath, entrypoint string, onUpload func()) *UploadHandler {
	return &UploadHandler{
		userCodePath:       userCodePath,
		userCodeEntrypoint: entrypoint,
		onUpload:           onUpload,
	}
}

func (h *UploadHandler) ProcessUpload(file multipart.File, header *multipart.FileHeader) error {
	filename := header.Filename

	if strings.HasSuffix(filename, ".py") {
		return h.processPythonFile(file)
	}

	if strings.HasSuffix(filename, ".zip") {
		return h.processZipFile(file, header.Size)
	}

	return fmt.Errorf("unsupported file type: %s", filename)
}

func (h *UploadHandler) processPythonFile(file multipart.File) error {
	tempDir, err := os.MkdirTemp("", "shepherd-user-code-")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}

	dst := filepath.Join(tempDir, h.userCodeEntrypoint)
	out, err := os.Create(dst)
	if err != nil {
		os.RemoveAll(tempDir)
		return fmt.Errorf("create temp file: %w", err)
	}

	if _, err := io.Copy(out, file); err != nil {
		out.Close()
		os.RemoveAll(tempDir)
		return fmt.Errorf("copy file: %w", err)
	}
	out.Close()

	return h.swapCodeDir(tempDir)
}

func (h *UploadHandler) processZipFile(file multipart.File, size int64) error {
	if size > maxExtractionSize {
		return fmt.Errorf("zip file too large (max %d bytes)", maxExtractionSize)
	}

	tempDir, err := os.MkdirTemp("", "shepherd-user-code-")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}

	z, err := zip.NewReader(file, size)
	if err != nil {
		os.RemoveAll(tempDir)
		return fmt.Errorf("read zip: %w", err)
	}

	var extracted int64
	for _, f := range z.File {
		dst := filepath.Join(tempDir, f.Name)

		if !strings.HasPrefix(filepath.Clean(dst), filepath.Clean(tempDir)+string(os.PathSeparator)) {
			os.RemoveAll(tempDir)
			return fmt.Errorf("zip entry %s escapes extraction directory", f.Name)
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(dst, 0755)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
			os.RemoveAll(tempDir)
			return fmt.Errorf("create dir for %s: %w", f.Name, err)
		}

		rc, err := f.Open()
		if err != nil {
			os.RemoveAll(tempDir)
			return fmt.Errorf("open zip entry %s: %w", f.Name, err)
		}

		out, err := os.Create(dst)
		if err != nil {
			rc.Close()
			os.RemoveAll(tempDir)
			return fmt.Errorf("create %s: %w", dst, err)
		}

		written, err := io.CopyN(out, rc, int64(f.UncompressedSize64))
		extracted += written
		out.Close()
		rc.Close()

		if extracted > maxExtractionSize {
			os.RemoveAll(tempDir)
			return fmt.Errorf("extracted zip exceeds maximum size")
		}

		if err != nil {
			os.RemoveAll(tempDir)
			return fmt.Errorf("extract %s: %w", f.Name, err)
		}
	}

	entrypoint := filepath.Join(tempDir, h.userCodeEntrypoint)
	if _, err := os.Stat(entrypoint); errors.Is(err, os.ErrNotExist) {
		os.RemoveAll(tempDir)
		return fmt.Errorf("zip file must contain a %s file at the root", h.userCodeEntrypoint)
	}

	return h.swapCodeDir(tempDir)
}

func (h *UploadHandler) swapCodeDir(tempDir string) error {
	oldDir := h.userCodePath + ".old"

	if err := os.RemoveAll(oldDir); err != nil {
		log.Printf("Warning: could not remove old backup: %v", err)
	}

	if _, err := os.Stat(h.userCodePath); err == nil {
		if err := os.Rename(h.userCodePath, oldDir); err != nil {
			os.RemoveAll(tempDir)
			return fmt.Errorf("backup old code: %w", err)
		}
	}

	if err := os.Rename(tempDir, h.userCodePath); err != nil {
		os.RemoveAll(oldDir)
		os.Rename(oldDir, h.userCodePath)
		return fmt.Errorf("swap code dir: %w", err)
	}

	os.RemoveAll(oldDir)

	if h.onUpload != nil {
		h.onUpload()
	}

	return nil
}
