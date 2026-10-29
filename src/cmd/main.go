package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"teto/internal/convert"
	"teto/internal/meta"
	"teto/internal/prepare"
)

type job struct {
	src    string
	rel    string
	dst    string
	ms     int
	listed bool
}

type result struct {
	job    job
	frames int
	err    error
}

func main() {
	in := flag.String("in", "emoticons/panorama/images/emoticons", "directory with the exported PNG strips")
	list := flag.String("meta", "emoticons", "emoticons list file, or a directory searched for them, with ms_per_frame")
	out := flag.String("out", "converted", "output directory")
	defMS := flag.Int("ms", 100, "ms per frame for images missing from emoticons.txt")
	workers := flag.Int("j", runtime.NumCPU(), "parallel workers")
	verbose := flag.Bool("v", false, "print every converted file")
	flag.Parse()

	timings, err := meta.LoadDir(*list)
	if err != nil {
		fmt.Fprintf(os.Stderr, "meta: %v\n", err)
		os.Exit(1)
	}

	files, err := prepare.GetFiles(*in)
	if err != nil {
		fmt.Fprintf(os.Stderr, "scan: %v\n", err)
		os.Exit(1)
	}

	jobs, err := plan(files, *in, *out, timings, *defMS)
	if err != nil {
		fmt.Fprintf(os.Stderr, "plan: %v\n", err)
		os.Exit(1)
	}

	var (
		converted, fromList, frames int
		skipped, failed             []result
	)
	for r := range run(jobs, *workers) {
		switch {
		case errors.Is(r.err, convert.ErrNotStrip):
			skipped = append(skipped, r)
		case r.err != nil:
			failed = append(failed, r)
		default:
			converted++
			frames += r.frames
			if r.job.listed {
				fromList++
			}
			if *verbose {
				src := "default"
				if r.job.listed {
					src = "list"
				}
				fmt.Printf("%s -> %s (%d frames, %d ms, %s)\n", r.job.rel, r.job.dst, r.frames, r.job.ms, src)
			}
		}
	}

	fmt.Printf("converted %d files, %d frames; %d with ms_per_frame from emoticons.txt, %d with default %d ms\n",
		converted, frames, fromList, converted-fromList, *defMS)

	fmt.Printf("%d entries in the lists matched no PNG\n", len(timings)-matched(jobs, timings))

	report("skipped", skipped)
	report("failed", failed)
	if len(failed) > 0 {
		os.Exit(1)
	}
}

func plan(files []prepare.FileInfo, in, out string, timings map[string]int, defMS int) ([]job, error) {
	var jobs []job
	taken := make(map[string]string)

	for _, f := range files {
		if !strings.EqualFold(filepath.Ext(f.Name), ".png") {
			continue
		}

		rel, err := filepath.Rel(in, f.Path)
		if err != nil {
			return nil, err
		}

		stem := strings.TrimSuffix(strings.TrimSuffix(rel, filepath.Ext(rel)), "_png")
		dst := filepath.Join(out, stem+".webp")
		if prev, ok := taken[dst]; ok {
			return nil, fmt.Errorf("%s and %s both convert to %s", prev, rel, dst)
		}
		taken[dst] = rel

		j := job{src: f.Path, rel: filepath.ToSlash(rel), dst: dst, ms: defMS}
		if ms, ok := timings[meta.Key(j.rel)]; ok {
			j.ms, j.listed = ms, true
		}
		jobs = append(jobs, j)
	}

	return jobs, nil
}

func matched(jobs []job, timings map[string]int) int {
	n := 0
	for _, j := range jobs {
		if _, ok := timings[meta.Key(j.rel)]; ok {
			n++
		}
	}
	return n
}

func run(jobs []job, workers int) <-chan result {
	in := make(chan job)
	out := make(chan result)

	var wg sync.WaitGroup
	for i := 0; i < max(workers, 1); i++ {
		wg.Go(func() {
			for j := range in {
				n, err := convert.File(j.src, j.dst, j.ms)
				out <- result{job: j, frames: n, err: err}
			}
		})
	}

	go func() {
		for _, j := range jobs {
			in <- j
		}
		close(in)
		wg.Wait()
		close(out)
	}()

	return out
}

func report(title string, rs []result) {
	if len(rs) == 0 {
		return
	}

	sort.Slice(rs, func(i, j int) bool { return rs[i].job.rel < rs[j].job.rel })
	fmt.Printf("%s (%d):\n", title, len(rs))
	for _, r := range rs {
		fmt.Printf("  %s: %v\n", r.job.rel, r.err)
	}
}
