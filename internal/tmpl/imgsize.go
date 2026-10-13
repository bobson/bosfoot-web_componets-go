package tmpl

import (
	"encoding/binary"
	"fmt"
	"html/template"
	"image"
	_ "image/jpeg" // register decoders for image.DecodeConfig fallback
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// imgSize returns `width="W" height="H"` for a public image, read from the
// file header (WebP parsed here; PNG/JPEG via image.DecodeConfig). Templates
// put it on <img> so the browser reserves the photo's REAL proportions before
// it loads — product photos keep their original aspect (Freet square, Be Lenka
// 3:4), so a hardcoded 1000x1000 would make the page jump. Cached per
// path+modtime+size like asset(). Unknown/missing files emit nothing.
func (r *Renderer) imgSize(path string) template.HTMLAttr {
	fullPath := filepath.Join("public", strings.TrimPrefix(path, "/"))
	info, err := os.Stat(fullPath)
	if err != nil {
		return ""
	}
	key := fmt.Sprintf("size:%s:%d:%d", path, info.ModTime().Unix(), info.Size())
	if v, ok := r.hashes.Load(key); ok {
		return v.(template.HTMLAttr)
	}
	f, err := os.Open(fullPath)
	if err != nil {
		return ""
	}
	defer f.Close()
	w, h, err := imageDims(f)
	if err != nil {
		return ""
	}
	attr := template.HTMLAttr(fmt.Sprintf(`width="%d" height="%d"`, w, h))
	r.hashes.Store(key, attr)
	return attr
}

// imageDims reads an image's pixel size from its header without decoding it.
func imageDims(rs io.ReadSeeker) (int, int, error) {
	hdr := make([]byte, 30)
	n, _ := io.ReadFull(rs, hdr)
	if w, h, ok := webpDims(hdr[:n]); ok {
		return w, h, nil
	}
	if _, err := rs.Seek(0, io.SeekStart); err != nil {
		return 0, 0, err
	}
	cfg, _, err := image.DecodeConfig(rs)
	if err != nil {
		return 0, 0, err
	}
	return cfg.Width, cfg.Height, nil
}

// webpDims parses the first chunk of a RIFF/WEBP file (VP8X, lossy VP8, or
// lossless VP8L) for the canvas size. ok=false if b isn't a WebP header.
func webpDims(b []byte) (w, h int, ok bool) {
	if len(b) < 30 || string(b[0:4]) != "RIFF" || string(b[8:12]) != "WEBP" {
		return 0, 0, false
	}
	le24 := func(p []byte) int { return int(p[0]) | int(p[1])<<8 | int(p[2])<<16 }
	switch string(b[12:16]) {
	case "VP8X": // extended: 24-bit (width-1), (height-1) at 24 and 27
		return le24(b[24:27]) + 1, le24(b[27:30]) + 1, true
	case "VP8 ": // lossy: keyframe start code 9d 01 2a, then 14-bit w/h
		if b[23] != 0x9d || b[24] != 0x01 || b[25] != 0x2a {
			return 0, 0, false
		}
		return int(binary.LittleEndian.Uint16(b[26:28]) & 0x3fff),
			int(binary.LittleEndian.Uint16(b[28:30]) & 0x3fff), true
	case "VP8L": // lossless: signature 0x2f, then 14-bit (w-1), 14-bit (h-1)
		if b[20] != 0x2f {
			return 0, 0, false
		}
		v := binary.LittleEndian.Uint32(b[21:25])
		return int(v&0x3fff) + 1, int((v>>14)&0x3fff) + 1, true
	}
	return 0, 0, false
}
