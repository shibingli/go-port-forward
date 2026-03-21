//go:build windows

package machineid

import (
	"golang.org/x/sys/windows/registry"
)

// machineID 从注册表 HKEY_LOCAL_MACHINE\SOFTWARE\Microsoft\Cryptography 读取 MachineGuid。
// 使用 WOW64_64KEY 标志确保在 32 位和 64 位进程中都能正确访问 64 位注册表视图。
//
// machineID returns the MachineGuid from registry `HKEY_LOCAL_MACHINE\SOFTWARE\Microsoft\Cryptography`.
// Uses WOW64_64KEY flag to ensure correct access to 64-bit registry view from both 32-bit and 64-bit processes.
func machineID() (string, error) {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Cryptography`, registry.QUERY_VALUE|registry.WOW64_64KEY)
	if err != nil {
		return "", err
	}
	defer func() { _ = k.Close() }()

	s, _, err := k.GetStringValue("MachineGuid")
	if err != nil {
		return "", err
	}
	return s, nil
}
