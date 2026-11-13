//go:build !windows

package config

import "os"

// restrictFileOwner 将文件权限收紧为仅属主可访问。
func restrictFileOwner(path string) {
	_ = os.Chmod(path, 0o600)
}
