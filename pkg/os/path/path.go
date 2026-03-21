// Package path 提供路径工具（工具箱容器模式）
// Package path provides path utilities (toolbox container mode)
package path

import (
	"os"
	"path/filepath"
	"runtime"
)

// GetDefaultSlurmBinPath 获取默认的 Slurm 二进制路径（容器内路径）
// Get default Slurm binary path (container-internal path)
func GetDefaultSlurmBinPath() string {
	return "/usr/bin"
}

// GetDefaultScratchMount 获取默认的 Scratch 挂载点（宿主机路径，toolbox data_dir 下）
// Get default Scratch mount point (host path, under toolbox data_dir)
func GetDefaultScratchMount() string {
	return "/opt/hpc-mgr/data/storage/scratch"
}

// GetDefaultHomeMount 获取默认的 Home 挂载点（宿主机路径，toolbox data_dir 下）
// Get default Home mount point (host path, under toolbox data_dir)
func GetDefaultHomeMount() string {
	return "/opt/hpc-mgr/data/storage/home"
}

// GetDefaultImagesMount 获取默认的 Images 挂载点（宿主机路径，toolbox data_dir 下）
// Get default Images mount point (host path, under toolbox data_dir)
// 对应 apptainer 容器卷: /opt/hpc-mgr/data/storage/images:/images
// Corresponds to apptainer container volume: /opt/hpc-mgr/data/storage/images:/images
func GetDefaultImagesMount() string {
	return "/opt/hpc-mgr/data/storage/images"
}

// GetDefaultModelsMount 获取默认的模型存储根目录（宿主机路径）
// Get default models storage root directory (host path)
func GetDefaultModelsMount() string {
	return "/opt/hpc-mgr/data/storage/models"
}

// GetDefaultPrometheusFileSDDir 获取默认的 Prometheus 文件服务发现目录（宿主机路径）
// Get default Prometheus file service discovery directory (host path)
// 应用写入此宿主机路径，prometheus容器通过卷挂载读取
// App writes to this host path, prometheus container reads via volume mount
// 对应容器卷: /opt/hpc-mgr/data/prometheus/config:/etc/prometheus
// Corresponds to container volume: /opt/hpc-mgr/data/prometheus/config:/etc/prometheus
func GetDefaultPrometheusFileSDDir() string {
	return "/opt/hpc-mgr/data/prometheus/config/file_sd"
}

// GetDefaultLogPath 获取默认的日志文件路径 | Get default log file path
func GetDefaultLogPath() string {
	switch runtime.GOOS {
	case "windows":
		return filepath.Join(os.Getenv("ProgramData"), "go-port-forward", "logs", "app.log")
	case "darwin":
		return "/var/log/go-port-forward/app.log"
	default: // linux
		return "/var/log/go-port-forward/app.log"
	}
}

// GetDefaultOfflinePackagesDir 获取默认的离线包存储根目录（宿主机路径）
// Get default offline packages storage root directory (host path)
// 用于无网络环境下的软件包安装 | Used for air-gapped software package installation
// 子目录: pip/ collections/ binary/ deb/ rpm/ | Subdirs: pip/ collections/ binary/ deb/ rpm/
func GetDefaultOfflinePackagesDir() string {
	return "/opt/hpc-mgr/data/storage/offline-packages"
}

// GetDefaultAnsiblePlaybookDir 获取默认的 Ansible Playbook 目录（容器内路径）
// Get default Ansible Playbook directory (container-internal path)
// 对应容器卷: /opt/hpc-mgr/data/ansible/playbooks:/workspace/playbooks
// Corresponds to container volume: /opt/hpc-mgr/data/ansible/playbooks:/workspace/playbooks
func GetDefaultAnsiblePlaybookDir() string {
	return "/workspace/playbooks"
}

// GetDefaultAnsibleRolesDir 获取默认的 Ansible Roles 目录（容器内路径）
// Get default Ansible Roles directory (container-internal path)
// 对应容器卷: /opt/hpc-mgr/data/ansible/roles:/workspace/roles
// Corresponds to container volume: /opt/hpc-mgr/data/ansible/roles:/workspace/roles
func GetDefaultAnsibleRolesDir() string {
	return "/workspace/roles"
}

// IsPathAbsolute 检查路径是否为绝对路径（跨平台）| Check if path is absolute (cross-platform)
func IsPathAbsolute(path string) bool {
	return filepath.IsAbs(path)
}

// NormalizePath 标准化路径（跨平台）| Normalize path (cross-platform)
func NormalizePath(path string) string {
	return filepath.Clean(path)
}

// JoinPath 连接路径（跨平台）| Join paths (cross-platform)
func JoinPath(elem ...string) string {
	return filepath.Join(elem...)
}

// IsPlatformSupported 检查当前平台是否支持指定功能（工具箱模式下全平台支持）
// Check if current platform supports specified feature (all platforms supported via toolbox mode)
// 所有工具通过 Docker 工具箱容器执行，不再依赖本地安装
// All tools execute in Docker toolbox containers, no longer depend on local installation
func IsPlatformSupported(_ string) bool {
	return true
}

// GetPlatformName 获取当前平台名称 | Get current platform name
func GetPlatformName() string {
	return runtime.GOOS
}

// IsWindows 检查是否为 Windows 平台 | Check if Windows platform
func IsWindows() bool {
	return runtime.GOOS == "windows"
}

// IsLinux 检查是否为 Linux 平台 | Check if Linux platform
func IsLinux() bool {
	return runtime.GOOS == "linux"
}

// IsDarwin 检查是否为 macOS 平台 | Check if macOS platform
func IsDarwin() bool {
	return runtime.GOOS == "darwin"
}
