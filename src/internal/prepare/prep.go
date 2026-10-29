package prepare

import (
	"io/fs"
	"path/filepath"
)

type FileInfo struct {
	Name string
	Path string
}

func GetFiles(rootDir string) ([]FileInfo, error) {
	var results []FileInfo

	err := filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		results = append(results, FileInfo{
			Name: d.Name(),
			Path: filepath.ToSlash(path),
		})

		return nil
	})

	return results, err
}
