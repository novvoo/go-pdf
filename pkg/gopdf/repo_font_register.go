package gopdf

import (
	"os"
	"sync"
)

var repoFontRegisterOnce sync.Map

func ensureRepoFontRegistered(key string, filePath string) {
	if key == "" || filePath == "" {
		return
	}
	if _, loaded := repoFontRegisterOnce.LoadOrStore(key, struct{}{}); loaded {
		return
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		return
	}
	_ = RegisterFontData(key, data)
}
