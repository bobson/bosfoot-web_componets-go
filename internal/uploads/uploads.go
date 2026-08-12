// Package uploads handles user-submitted review photos: validating them,
// converting to a small stripped WebP via ImageMagick, and moving them through
// the moderation lifecycle. Files live OUTSIDE the repo (UPLOADS_DIR) so the
// git-reset deploy doesn't wipe them, and pending photos live in an UNSERVED
// directory — only approved photos are moved into the web-served public dir.
//
// Layout under UPLOADS_DIR:
//
//	pending/reviews/<name>.webp   uploaded, awaiting moderation — NOT web-served
//	public/reviews/<name>.webp    approved — served at /uploads/reviews/<name>.webp
package uploads

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/gif"  // register decoders for DecodeConfig (bomb/format check)
	_ "image/jpeg" //
	_ "image/png"  //
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	// MaxFileBytes caps a single uploaded file before any decoding.
	MaxFileBytes = 12 << 20 // 12 MiB
	// MaxPixels rejects decompression bombs (before handing to ImageMagick) when
	// the format is one Go can inspect.
	MaxPixels = 40 * 1000 * 1000 // 40 MP
	// MaxPhotos is the per-review photo cap.
	MaxPhotos = 3
	maxDim    = "1280x1280>" // ImageMagick: shrink to fit, never enlarge
)

// Dir is the base uploads directory (UPLOADS_DIR env, default "uploads").
func Dir() string {
	if d := strings.TrimSpace(os.Getenv("UPLOADS_DIR")); d != "" {
		return d
	}
	return "uploads"
}

// PublicRoot is the directory the web server serves at /uploads/ (approved files
// only). e.g. <dir>/public, so /uploads/reviews/x.webp -> <dir>/public/reviews/x.webp.
func PublicRoot() string { return filepath.Join(Dir(), "public") }

func pendingPath(name string) string { return filepath.Join(Dir(), "pending", "reviews", name) }
func publicPath(name string) string  { return filepath.Join(Dir(), "public", "reviews", name) }

// PublicURL is the browser URL for an approved review photo.
func PublicURL(name string) string { return "/uploads/reviews/" + name }

// PendingFsPath is the on-disk path of a not-yet-approved photo, for the
// moderator to open (cmd/reviews -photo).
func PendingFsPath(name string) string { return pendingPath(name) }

// magickBin finds the ImageMagick CLI, or "" if not installed. When absent, the
// caller saves the review text and skips the photo rather than failing.
func magickBin() string {
	for _, b := range []string{"magick", "convert"} {
		if p, err := exec.LookPath(b); err == nil {
			return p
		}
	}
	return ""
}

// Available reports whether photo conversion is possible (ImageMagick present).
func Available() bool { return magickBin() != "" }

// SaveReviewPhoto validates src, converts it to a stripped ≤1280px WebP, writes
// it to the PENDING dir, and returns the generated filename. Never trusts the
// client filename. Returns an error the caller can log-and-skip (bad image, no
// ImageMagick, etc.) without failing the whole review.
func SaveReviewPhoto(src io.Reader) (string, error) {
	bin := magickBin()
	if bin == "" {
		return "", fmt.Errorf("imagemagick not installed")
	}

	// 1) Copy to a temp file with a hard size cap (we name the file, never the
	//    client) so nothing untrusted reaches the shell.
	tmp, err := os.CreateTemp("", "revphoto-*")
	if err != nil {
		return "", err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	n, err := io.Copy(tmp, io.LimitReader(src, MaxFileBytes+1))
	tmp.Close()
	if err != nil {
		return "", err
	}
	if n > MaxFileBytes {
		return "", fmt.Errorf("file too large (max %d bytes)", MaxFileBytes)
	}

	// 2) Format + decompression-bomb gate for formats Go can inspect (jpeg/png/
	//    gif). Unknown formats (e.g. HEIC/WebP) fall through to ImageMagick, whose
	//    memory limits below bound the blast radius.
	if f, e := os.Open(tmpName); e == nil {
		cfg, _, derr := image.DecodeConfig(f)
		f.Close()
		if derr == nil && cfg.Width*cfg.Height > MaxPixels {
			return "", fmt.Errorf("image too large (%dx%d)", cfg.Width, cfg.Height)
		}
	}

	// 3) Convert -> WebP into the pending dir with strict limits + timeout.
	name, err := randName()
	if err != nil {
		return "", err
	}
	out := pendingPath(name)
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	// `<tmp>[0]` = first frame only (animated inputs); -strip removes EXIF/GPS.
	args := []string{
		"-limit", "memory", "256MiB", "-limit", "map", "512MiB",
		tmpName + "[0]", "-auto-orient", "-strip", "-resize", maxDim,
		"-quality", "80", "webp:" + out,
	}
	cmd := exec.CommandContext(ctx, bin, args...)
	if outp, err := cmd.CombinedOutput(); err != nil {
		os.Remove(out)
		return "", fmt.Errorf("convert failed: %v: %s", err, strings.TrimSpace(string(outp)))
	}
	return name, nil
}

// Publish moves an approved photo from pending -> public (idempotent-ish: if it's
// already public, it's a no-op).
func Publish(name string) error {
	if !safeName(name) {
		return fmt.Errorf("bad name")
	}
	src, dst := pendingPath(name), publicPath(name)
	if _, err := os.Stat(dst); err == nil {
		return nil // already published
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.Rename(src, dst)
}

// Remove deletes a photo file from wherever it is (pending and/or public). Used
// on reject/delete so moderated-out content never lingers on disk. Missing files
// are not an error.
func Remove(name string) error {
	if !safeName(name) {
		return fmt.Errorf("bad name")
	}
	for _, p := range []string{pendingPath(name), publicPath(name)} {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func randName() (string, error) {
	b := make([]byte, 18)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b) + ".webp", nil
}

// safeName guards path operations against traversal — filenames we generate are
// base64url + ".webp", so anything with a slash or "." sequence is rejected.
func safeName(name string) bool {
	return name != "" && !strings.ContainsAny(name, "/\\") && !strings.Contains(name, "..") &&
		strings.HasSuffix(name, ".webp")
}
