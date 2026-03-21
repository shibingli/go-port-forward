# Go Port Forward

高性能跨平台 TCP/UDP 端口转发工具，内置 Web 管理界面。

A high-performance cross-platform TCP/UDP port forwarder with a built-in Web UI.

## ✨ 功能特性 | Features

- **TCP / UDP / Both** 端口转发，支持同时转发双协议
- **Web 管理界面** — 基于 Alpine.js + Bootstrap 5 的现代化单页应用
- **WSL2 端口导入** — 自动发现 WSL2 发行版监听端口并一键导入转发规则
- **跨平台防火墙管理** — Windows (netsh)、Linux (iptables)、macOS (pfctl) 自动添加/删除防火墙规则
- **系统服务支持** — 可注册为 Windows Service / Linux systemd / macOS launchd 后台服务
- **高性能并发** — 基于 [ants](https://github.com/panjf2000/ants) 协程池，支持万级并发连接
- **嵌入式存储** — 使用 [bbolt](https://go.etcd.io/bbolt) 嵌入式 KV 数据库，零依赖部署
- **自动 GC 管理** — 内存阈值触发 + 定时 GC，多种回收策略可选
- **YAML 配置** — 首次运行自动生成默认配置文件

## 📦 项目结构 | Project Structure

```
go-port-forward/
├── main.go                  # 程序入口 | Entry point
├── config.yaml              # 配置文件 | Configuration
├── internal/
│   ├── config/              # 配置加载 (Viper) | Config loading
│   ├── firewall/            # 跨平台防火墙管理 | Cross-platform firewall
│   │   ├── firewall.go      # 接口定义 | Interface
│   │   ├── firewall_windows.go
│   │   ├── firewall_linux.go
│   │   └── firewall_darwin.go
│   ├── forward/             # 转发核心 | Forwarding core
│   │   ├── manager.go       # 规则生命周期管理 | Rule lifecycle
│   │   ├── tcp.go           # TCP 转发器 | TCP forwarder
│   │   └── udp.go           # UDP 转发器 | UDP forwarder
│   ├── logger/              # 日志初始化 | Logger init
│   ├── models/              # 数据模型 | Data models
│   ├── storage/             # bbolt 持久化 | bbolt persistence
│   ├── svc/                 # 系统服务封装 | System service wrapper
│   └── web/                 # Web 服务 + 嵌入式静态资源 | Web server + embedded static
│       ├── server.go
│       ├── handlers.go
│       ├── handlers_wsl.go
│       └── static/          # 前端资源 (Alpine.js, Bootstrap, HTMX)
├── pkg/
│   ├── gc/                  # GC 管理服务 | GC management
│   ├── pool/                # 协程池封装 (ants) | Goroutine pool
│   ├── retry/               # 重试机制 | Retry utilities
│   ├── logger/              # 全局日志桥接 | Global logger bridge
│   ├── serializer/          # JSON 序列化 (sonic/jsoniter) | JSON serialization
│   └── os/                  # OS 工具 (WSL 发现等) | OS utilities
└── data/
    └── rules.db             # bbolt 数据库文件 | Database file
```

## 🚀 快速开始 | Quick Start

### 编译 | Build

```bash
go build -o go-port-forward .
```

### 运行 | Run

```bash
# 前台运行 | Foreground
./go-port-forward

# 指定配置文件 | With custom config
./go-port-forward -config /path/to/config.yaml
```

启动后访问 `http://127.0.0.1:8080` 打开 Web 管理界面。

### 系统服务 | System Service

```bash
# 安装为系统服务 | Install as system service
./go-port-forward -service install

# 以服务方式运行 | Run as service
./go-port-forward -service run

# 卸载服务 | Uninstall service
./go-port-forward -service uninstall
```

## ⚙️ 配置 | Configuration

首次运行时会在可执行文件同目录自动生成 `config.yaml`：

```yaml
web:
  host: 127.0.0.1          # Web UI 监听地址
  port: 8080                # Web UI 端口
  # username: admin         # Basic Auth 用户名 (留空禁用)
  # password: secret        # Basic Auth 密码

storage:
  path: data/rules.db       # 数据库路径

log:
  level: info               # 日志级别: debug | info | warn | error
  path: logs/app.log        # 日志文件路径
  max_size_mb: 50           # 单文件最大 MB
  max_backups: 5            # 保留备份数
  max_age_days: 30          # 保留天数
  compress: true            # 压缩归档

forward:
  buffer_size: 32768        # I/O 缓冲区大小 (bytes)
  dial_timeout: 10          # 出站连接超时 (秒)
  udp_timeout: 30           # UDP 会话空闲超时 (秒)
  pool_size: 0              # 协程池大小 (0 = 自动)

gc:
  enabled: true
  interval_seconds: 300     # GC 间隔 (秒)
  strategy: standard        # GC 策略: standard | aggressive | conservative
  memory_threshold_mb: 100  # 内存阈值 (MB)
  enable_monitoring: true
```

## 🔌 REST API

| 方法 | 路径 | 描述 |
|------|------|------|
| `GET` | `/api/rules` | 列出所有转发规则 |
| `POST` | `/api/rules` | 创建转发规则 |
| `GET` | `/api/rules/{id}` | 获取单条规则 |
| `PUT` | `/api/rules/{id}` | 更新规则 |
| `DELETE` | `/api/rules/{id}` | 删除规则 |
| `PUT` | `/api/rules/{id}/toggle` | 启用/禁用规则 |
| `GET` | `/api/stats` | 获取全局统计 |
| `GET` | `/api/wsl/distros` | 列出 WSL2 发行版 |
| `GET` | `/api/wsl/ports/{distro}` | 列出发行版监听端口 |
| `POST` | `/api/wsl/import` | 批量导入 WSL2 端口 |

## 📋 系统要求 | Requirements

- **Go** 1.26+
- **Windows** / **Linux** / **macOS**
- 防火墙管理需要管理员/root 权限

## 📄 License

MIT

