// Package machineid 提供跨平台读取主机唯一机器标识的能力（无需管理员权限）。
//
// Package machineid provides support for reading the unique machine id of most OSs (without admin privileges).
//
// 本包 vendored 自 github.com/panta/machineid（fork of github.com/denisbrodbeck/machineid），由项目自行维护。
// This package is a vendored copy of github.com/panta/machineid, maintained in-tree.
// Source: https://github.com/panta/machineid
//
// 跨平台支持 | Cross-Platform: Win7+, Debian 8+, Ubuntu 14.04+, OS X 10.6+, FreeBSD 11+
// 不使用任何硬件标识（无 MAC/BIOS/CPU）| No internal hardware IDs (no MAC, BIOS, or CPU).
//
// 返回的 machine ID 在 OS 安装后保持稳定，通常在系统更新或硬件变更后不会变化。
// Returned machine IDs are generally stable for the OS installation
// and usually stay the same after updates or hardware changes.
//
// 注意：基于镜像的环境通常具有相同的 machine-id（完美克隆）。
// Caveat: Image-based environments have usually the same machine-id (perfect clone).
// Linux: 可通过 dbus-uuidgen 生成新 ID 并写入 /var/lib/dbus/machine-id 和 /etc/machine-id。
// Windows: 使用 sysprep 工具链创建部署镜像以生成新的 MachineGuid。
package machineid

import (
	"errors"
	"fmt"
	"sync"
)

var (
	// ErrEmptyID 当平台返回空的 machine ID 时使用此哨兵错误。
	// ErrEmptyID is returned when the platform provides an empty machine ID.
	ErrEmptyID = errors.New("machineid: empty machine id")

	once      sync.Once
	cachedID  string
	cachedErr error
)

// ID 返回当前主机操作系统的平台特定 machine id。
// 结果在首次成功调用后缓存（machine ID 在进程生命周期内稳定不变）。
// 返回的 ID 应视为"机密"信息，建议使用 ProtectedID() 获取哈希后的安全版本。
//
// ID returns the platform specific machine id of the current host OS.
// The result is cached after the first call (machine ID is stable within process lifetime).
// Regard the returned id as "confidential" and consider using ProtectedID() instead.
func ID() (string, error) {
	once.Do(func() {
		id, err := machineID()
		if err != nil {
			cachedErr = fmt.Errorf("machineid: %w", err)
			return
		}
		if id == "" {
			cachedErr = ErrEmptyID
			return
		}
		cachedID = id
	})
	return cachedID, cachedErr
}

// ProtectedID 以密码学安全的方式返回 machine ID 的哈希版本。
// 内部使用 HMAC-SHA256，以 machine ID 为密钥对 appID 进行哈希计算。
// appID 不允许为空。
//
// ProtectedID returns a hashed version of the machine ID in a cryptographically secure way,
// using a fixed, application-specific key.
// Internally, this function calculates HMAC-SHA256 of the application ID, keyed by the machine ID.
// appID must not be empty.
func ProtectedID(appID string) (string, error) {
	if appID == "" {
		return "", errors.New("machineid: appID must not be empty")
	}
	id, err := ID()
	if err != nil {
		return "", err // 错误已由 ID() 包装 | error already wrapped by ID()
	}
	return protect(appID, id), nil
}
