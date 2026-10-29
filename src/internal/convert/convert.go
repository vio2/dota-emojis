package convert

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"

	"github.com/HugoSmits86/nativewebp"
)

var ErrNotStrip = errors.New("not a frame strip")

var opts = &nativewebp.Options{CompressionLevel: nativewebp.BestCompression}

func File(src, dst string, frameMS int) (int, error) {
	f, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	img, err := png.Decode(f)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", src, err)
	}

	frames, err := split(img)
	if err != nil {
		return 0, err
	}

	var buf bytes.Buffer
	if len(frames) == 1 {
		err = nativewebp.Encode(&buf, frames[0], opts)
	} else {
		err = encodeAnimation(&buf, frames, frameMS)
	}
	if err != nil {
		return 0, fmt.Errorf("%s: %w", src, err)
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return 0, err
	}

	return len(frames), os.WriteFile(dst, buf.Bytes(), 0o644)
}

func encodeAnimation(buf *bytes.Buffer, frames []image.Image, frameMS int) error {
	ani := nativewebp.Animation{
		Images:    frames,
		Durations: make([]uint, len(frames)),
		Disposals: make([]uint, len(frames)),
	}
	for i := range frames {
		ani.Durations[i] = uint(frameMS)
		ani.Disposals[i] = 1
	}

	return nativewebp.EncodeAll(buf, &ani, opts)
}

func split(img image.Image) ([]image.Image, error) {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if h == 0 || w < h || w%h != 0 {
		return nil, fmt.Errorf("%w: %dx%d", ErrNotStrip, w, h)
	}

	frames := make([]image.Image, 0, w/h)
	for x := 0; x < w; x += h {
		frames = append(frames, crop(img, image.Rect(b.Min.X+x, b.Min.Y, b.Min.X+x+h, b.Max.Y)))
	}

	return frames, nil
}

func crop(img image.Image, r image.Rectangle) *image.NRGBA {
	dst := image.NewNRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))

	if src, ok := img.(*image.NRGBA); ok {
		for y := 0; y < r.Dy(); y++ {
			from := src.PixOffset(r.Min.X, r.Min.Y+y)
			copy(dst.Pix[y*dst.Stride:y*dst.Stride+r.Dx()*4], src.Pix[from:from+r.Dx()*4])
		}
		return dst
	}

	for y := 0; y < r.Dy(); y++ {
		for x := 0; x < r.Dx(); x++ {
			dst.Set(x, y, img.At(r.Min.X+x, r.Min.Y+y))
		}
	}

	return dst
}
