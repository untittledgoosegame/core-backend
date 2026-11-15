package user

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
)

func encode(t *testing.T, w, h int, asJPEG bool) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := range w {
		for y := range h {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 100, 255})
		}
	}
	var buf bytes.Buffer
	var err error
	if asJPEG {
		err = jpeg.Encode(&buf, img, nil)
	} else {
		err = png.Encode(&buf, img)
	}
	if err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestProcessAvatar(t *testing.T) {
	data, ct, ext, err := processAvatar(encode(t, 1200, 800, true))
	if err != nil {
		t.Fatal(err)
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || ct != "image/jpeg" || ext != "jpg" || format != "jpeg" {
		t.Fatalf("got %s %s %s, %v", ct, ext, format, err)
	}
	if cfg.Width != avatarSide || cfg.Height != avatarSide {
		t.Fatalf("got %dx%d, want %dx%d", cfg.Width, cfg.Height, avatarSide, avatarSide)
	}

	data, ct, _, err = processAvatar(encode(t, 100, 300, false))
	if err != nil || ct != "image/png" {
		t.Fatalf("png: %s, %v", ct, err)
	}
	if cfg, _, _ := image.DecodeConfig(bytes.NewReader(data)); cfg.Width != 100 || cfg.Height != 100 {
		t.Fatalf("small image should be cropped, not upscaled: %dx%d", cfg.Width, cfg.Height)
	}

	if _, _, _, err := processAvatar([]byte("<svg onload=alert(1)>")); err != errNotImage {
		t.Fatalf("non-image: %v", err)
	}

	bomb := image.NewGray(image.Rect(0, 0, 10000, 10000))
	var buf bytes.Buffer
	_ = png.Encode(&buf, bomb)
	if _, _, _, err := processAvatar(buf.Bytes()); err != errTooManyPixels {
		t.Fatalf("pixel bomb (%d bytes): %v", buf.Len(), err)
	}
}
