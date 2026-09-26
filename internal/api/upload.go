package api

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"

	"github.com/google/uuid"
)

var allowedImageTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
}

// saveUploadedImage validates and persists an uploaded image, returning
// its storage path (relative to UploadsDir, safe to store in the DB and
// safe to join back with UploadsDir later — never derived from anything
// the client sent, so there's no path-traversal surface).
func (s *Server) saveUploadedImage(subdir string, fh *multipart.FileHeader) (relPath, contentType string, size int64, err error) {
	if fh.Size > s.cfg.MaxUploadBytes {
		return "", "", 0, fmt.Errorf("file too large: max %d bytes", s.cfg.MaxUploadBytes)
	}
	f, err := fh.Open()
	if err != nil {
		return "", "", 0, err
	}
	defer f.Close()

	head := make([]byte, 512)
	n, _ := io.ReadFull(f, head)
	head = head[:n]
	sniffed := http.DetectContentType(head)
	ext, ok := allowedImageTypes[sniffed]
	if !ok {
		return "", "", 0, fmt.Errorf("unsupported file type %q — only JPEG, PNG, or WebP images are accepted", sniffed)
	}

	dir := filepath.Join(s.cfg.UploadsDir, subdir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", "", 0, err
	}
	name := uuid.NewString() + ext
	relPath = filepath.ToSlash(filepath.Join(subdir, name))
	fullPath := filepath.Join(s.cfg.UploadsDir, subdir, name)

	out, err := os.Create(fullPath)
	if err != nil {
		return "", "", 0, err
	}
	defer out.Close()

	written, err := io.Copy(out, io.MultiReader(bytes.NewReader(head), f))
	if err != nil {
		os.Remove(fullPath)
		return "", "", 0, err
	}
	return relPath, sniffed, written, nil
}
