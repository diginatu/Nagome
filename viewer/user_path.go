// +build !windows

package viewer

import (
	"os"
	"path/filepath"
)

func findUserConfigPath(appname string) string {
	home := os.Getenv("HOME")
	dir := filepath.Join(home, ".config", appname)

	return dir
}
