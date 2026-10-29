package upscale

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

type Options struct {
	Size     int
	GPU      bool
	Scaler   string
	Lossless bool
}

func File(src, dst string, o Options) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	scale := fmt.Sprintf("scale=%d:%d:flags=lanczos", o.Size, o.Size)
	if o.GPU {
		scale = fmt.Sprintf("libplacebo=w=%d:h=%d:upscaler=%s:format=gbrap16le", o.Size, o.Size, o.Scaler)
	}
	filter := "format=gbrap16le,premultiply=inplace=1," + scale + ",unpremultiply=inplace=1,format=rgba"

	lossless := "0"
	if o.Lossless {
		lossless = "1"
	}

	cmd := exec.Command("ffmpeg", "-v", "error", "-y", "-i", src,
		"-fps_mode", "passthrough", "-vf", filter,
		"-c:v", "libwebp_anim", "-lossless", lossless, "-loop", "0", dst)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		os.Remove(dst)
		return fmt.Errorf("%s: %w: %s", src, err, bytes.TrimSpace(stderr.Bytes()))
	}

	return nil
}
