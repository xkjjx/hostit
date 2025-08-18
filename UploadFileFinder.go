package main

import (
	"errors"
	"os"
	"path/filepath"
)

type UploadFileFinder struct{}

func NewUploadFileFinder() *UploadFileFinder {
	return &UploadFileFinder{}
}

// FindFiles walks the provided folder and returns repo-relative paths (slash-normalized)
// for all regular files whose sizes are less than or equal to maxFileSizeBytes.
func (finder UploadFileFinder) FindFiles(folderName string, maxFileSizeBytes int64) ([]string, error) {
	if folderName == "" {
		return nil, errors.New("folderName must not be empty")
	}

	info, err := os.Stat(folderName)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, errors.New("path is not a directory")
	}

	var files []string
	walkErr := filepath.WalkDir(folderName, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			// Continue walking despite errors on individual entries
			return nil
		}
		if d.IsDir() {
			return nil
		}
		fileInfo, err := d.Info()
		if err != nil {
			return nil
		}
		if !fileInfo.Mode().IsRegular() {
			return nil
		}
		if maxFileSizeBytes > 0 && fileInfo.Size() > maxFileSizeBytes {
			return nil
		}
		relPath, err := filepath.Rel(folderName, path)
		if err != nil {
			return nil
		}
		repoPath := filepath.ToSlash(relPath)
		files = append(files, repoPath)
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}
	return files, nil
} 