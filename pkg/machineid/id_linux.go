//go:build linux

package machineid

import (
	"errors"
	"os"
	"path/filepath"
)

// EnvVarName 用于覆盖 machine-id 文件路径的环境变量名称。
// EnvVarName is the environment variable that can override the machine-id file path.
const EnvVarName = "MACHINE_ID_FILE"

const (
	// dbusPath dbus machine-id 的默认路径。
	// dbusPath is the default path for dbus machine id.
	dbusPath = "/var/lib/dbus/machine-id"

	// dbusPathEtc 位于 /etc 下的 dbus machine-id 路径。
	// 某些系统（如 Fedora 20）仅使用此路径，有时反过来。
	// dbusPathEtc is the default path for dbus machine id located in /etc.
	// Some systems (like Fedora 20) only know this path; sometimes it's the other way round.
	dbusPathEtc = "/etc/machine-id"

	// linuxRandomUUID 内核随机 UUID 生成器（每次读取返回新 UUID）。
	// linuxRandomUUID is the kernel random UUID generator (returns a new UUID on each read).
	linuxRandomUUID = "/proc/sys/kernel/random/uuid"
)

// machineID 按优先级从以下位置读取 machine id：
//  1. $MACHINE_ID_FILE 环境变量指向的文件
//  2. /var/lib/dbus/machine-id
//  3. /etc/machine-id
//  4. $XDG_CONFIG_HOME/machine-id（通常为 ~/.config/machine-id）
//
// 若所有文件均不可读，则从 /proc/sys/kernel/random/uuid 生成随机 UUID，
// 并尽力持久化到上述路径之一（若全部写入失败则 ID 不会跨重启保持一致）。
//
// machineID reads the machine id from canonical locations in priority order:
//  1. file pointed to by $MACHINE_ID_FILE env var
//  2. /var/lib/dbus/machine-id
//  3. /etc/machine-id
//  4. $XDG_CONFIG_HOME/machine-id (usually ~/.config/machine-id)
//
// If no file is found, a random UUID is generated from /proc/sys/kernel/random/uuid
// and persisted to the first writable path (ID won't survive reboot if all writes fail).
func machineID() (string, error) {
	envPathname := os.Getenv(EnvVarName)

	// 使用标准库获取用户配置目录，避免 HOME 未设置时产生相对路径
	// Use stdlib to get user config dir, avoiding relative path when HOME is unset
	var userMachineID string
	if configDir, err := os.UserConfigDir(); err == nil {
		userMachineID = filepath.Join(configDir, "machine-id")
	}

	data, err := readFirstFile([]string{envPathname, dbusPath, dbusPathEtc, userMachineID})
	if err != nil {
		// 所有候选文件均不可读，降级到内核随机 UUID 生成器
		// All candidate files unreadable, fall back to kernel random UUID generator
		data, err = readFile(linuxRandomUUID)
		if err != nil {
			return "", err
		}
		// 尽力持久化，忽略写入错误（如无权限写入系统路径）
		// Best-effort persistence; ignore write errors (e.g., no permission for system paths)
		_ = writeFirstFile([]string{envPathname, dbusPathEtc, dbusPath, userMachineID}, data)
	}

	id := trim(string(data))
	if id == "" {
		return "", errors.New("machine-id file exists but contains empty content")
	}
	return id, nil
}
