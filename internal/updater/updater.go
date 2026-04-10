package updater

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
)

// Updater handles atomic self-replacement after SHA256 verification.
type Updater struct {
	BinaryPath string
	HTTPClient *http.Client
}

// NewUpdater returns an updater that writes to binaryPath using the default HTTP client.
func NewUpdater(binaryPath string) *Updater {
	return &Updater{
		BinaryPath: binaryPath,
		HTTPClient: http.DefaultClient,
	}
}

// Update downloads the binary from downloadURL, verifies SHA256, and atomically
// replaces the binary at BinaryPath. On any error the existing binary is preserved.
func (u *Updater) Update(downloadURL, expectedSHA256 string) error {
	resp, err := u.HTTPClient.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download: HTTP %d", resp.StatusCode)
	}

	tmpPath := u.BinaryPath + ".new"
	tmpFile, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}

	hasher := sha256.New()
	writer := io.MultiWriter(tmpFile, hasher)
	if _, err := io.Copy(writer, resp.Body); err != nil {
		tmpFile.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("copy body: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close temp: %w", err)
	}

	actualSHA := hex.EncodeToString(hasher.Sum(nil))
	if actualSHA != expectedSHA256 {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("sha256 mismatch: got %s, want %s", actualSHA, expectedSHA256)
	}

	if err := os.Rename(tmpPath, u.BinaryPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}
