package tmpl

import (
	"os"
	"testing"
)

func TestImageDims(t *testing.T) {
	for _, name := range []string{
		"lossy-30x40.webp",    // VP8
		"lossless-30x40.webp", // VP8L
		"alpha-30x40.webp",    // VP8X
		"plain-30x40.png",     // image.DecodeConfig fallback
	} {
		f, err := os.Open("testdata/" + name)
		if err != nil {
			t.Fatal(err)
		}
		w, h, err := imageDims(f)
		f.Close()
		if err != nil || w != 30 || h != 40 {
			t.Errorf("%s: got %dx%d err=%v, want 30x40", name, w, h, err)
		}
	}
}
