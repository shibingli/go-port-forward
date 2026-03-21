//go:build freebsd || netbsd || openbsd || dragonfly || solaris

package machineid

import (
	"errors"
	"io"

	"go-port-forward/pkg/pool"
)

const hostidPath = "/etc/hostid"

// machineID 首先尝试读取 /etc/hostid，失败时降级调用 kenv（仅 FreeBSD 可用）。
// machineID reads the uuid from /etc/hostid first.
// If that fails, falls back to `kenv -q smbios.system.uuid` (FreeBSD only).
func machineID() (string, error) {
	id, err := readHostid()
	if err != nil {
		// 降级到 kenv（仅 FreeBSD 有效；NetBSD/OpenBSD/DragonFly/Solaris 无 kenv）
		// Fallback to kenv (FreeBSD only; not available on NetBSD/OpenBSD/DragonFly/Solaris)
		id, err = readKenv()
	}
	if err != nil {
		return "", err
	}
	if id == "" {
		return "", errors.New("empty machine id from hostid/kenv")
	}
	return id, nil
}

// readHostid 从 /etc/hostid 读取机器标识。
// readHostid reads the machine ID from /etc/hostid.
func readHostid() (string, error) {
	buf, err := readFile(hostidPath)
	if err != nil {
		return "", err
	}
	return trim(string(buf)), nil
}

// readKenv 通过 kenv 命令读取 smbios.system.uuid。
// 注意：kenv 是 FreeBSD 专有工具，在 NetBSD、OpenBSD、DragonFly BSD、Solaris 上不可用。
// stderr 写入 io.Discard：library 不应直接向进程 stderr 输出。
//
// readKenv reads smbios.system.uuid via the kenv command.
// Note: kenv is FreeBSD-specific and not available on NetBSD, OpenBSD, DragonFly BSD, or Solaris.
// stderr goes to io.Discard: libraries should not write directly to process stderr.
func readKenv() (string, error) {
	buf := pool.GetByteBuffer()
	defer pool.PutByteBuffer(buf)
	if err := run(buf, io.Discard, "kenv", "-q", "smbios.system.uuid"); err != nil {
		return "", err
	}
	return trim(buf.String()), nil
}
