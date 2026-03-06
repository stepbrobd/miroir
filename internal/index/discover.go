package index

import (
	"fmt"
	"os"
	"path/filepath"
)

// bare repo markers: HEAD file + objects/ dir
func isBareRepo(path string) bool {
	_, errH := os.Stat(filepath.Join(path, "HEAD"))
	info, errO := os.Stat(filepath.Join(path, "objects"))
	return errH == nil && errO == nil && info.IsDir()
}

func isNonBareRepo(path string) bool {
	info, err := os.Stat(filepath.Join(path, ".git"))
	return err == nil && info.IsDir()
}

// Discover finds git repos one level deep under each dir in paths
// no recursion, only direct children are examined
func Discover(paths []string) ([]string, error) {
	var repos []string
	for _, base := range paths {
		entries, err := os.ReadDir(base)
		if err != nil {
			return nil, fmt.Errorf("discover %s: %w", base, err)
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			p := filepath.Join(base, e.Name())
			if isBareRepo(p) || isNonBareRepo(p) {
				repos = append(repos, p)
			}
		}
	}
	return repos, nil
}
