//go:build windows

package config

import (
	"os"

	"golang.org/x/sys/windows/registry"
)

// windowsMachineGUID 读取注册表中的机器 GUID 作为机器标识。
func windowsMachineGUID() string {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE,
		`SOFTWARE\Microsoft\Cryptography`, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer k.Close()
	v, _, err := k.GetStringValue("MachineGuid")
	if err != nil {
		return ""
	}
	return v
}

// restrictFileOwner 将文件 ACL 收紧为仅当前用户可访问（尽力而为）。
func restrictFileOwner(path string) {
	_ = os.Chmod(path, 0o600)
}
