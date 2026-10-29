package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"teto/internal/prepare"
	"teto/internal/upscale"
)

type job struct {
	src string
	rel string
	dst string
}

func main() {
	in := flag.String("in", "converted", "directory with the converted WebP files")
	out := flag.String("out", "converted_512", "output directory")
	size := flag.Int("size", 512, "output width and height")
	gpu := flag.Bool("gpu", false, "scale on the GPU with libplacebo (works, but slower for tiny frames)")
	scaler := flag.String("scaler", "ewa_lanczos", "libplacebo upscaler used with -gpu, e.g. ewa_lanczos, spline36, nearest")
	lossless := flag.Bool("lossless", false, "lossless WebP, much bigger files")
	workers := flag.Int("j", runtime.NumCPU(), "parallel ffmpeg processes")
	flag.Parse()

	if _, err := os.Stat(*in); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	files, err := prepare.GetFiles(*in)
	if err != nil {
		fmt.Fprintf(os.Stderr, "scan: %v\n", err)
		os.Exit(1)
	}

	var jobs []job
	for _, f := range files {
		if !strings.EqualFold(filepath.Ext(f.Name), ".webp") {
			continue
		}
		rel, err := filepath.Rel(*in, f.Path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
		jobs = append(jobs, job{src: f.Path, rel: filepath.ToSlash(rel), dst: filepath.Join(*out, rel)})
	}

	opts := upscale.Options{Size: *size, GPU: *gpu, Scaler: *scaler, Lossless: *lossless}

	todo := make(chan job)
	var (
		wg     sync.WaitGroup
		mu     sync.Mutex
		failed []error
	)
	for i := 0; i < max(*workers, 1); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range todo {
				if err := upscale.File(j.src, j.dst, opts); err != nil {
					mu.Lock()
					failed = append(failed, err)
					mu.Unlock()
				}
			}
		}()
	}
	for _, j := range jobs {
		todo <- j
	}
	close(todo)
	wg.Wait()

	fmt.Printf("upscaled %d of %d files to %dx%d in %s\n", len(jobs)-len(failed), len(jobs), *size, *size, *out)
	if len(failed) > 0 {
		fmt.Printf("failed (%d):\n", len(failed))
		for _, err := range failed {
			fmt.Printf("  %v\n", err)
		}
		os.Exit(1)
	}
}
