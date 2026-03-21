# GC 垃圾回收管理器 | GC Manager

基于 `pkg/retry` 包的设计模式，为系统提供智能化、自动化的GC管理功能。
Based on the `pkg/retry` package design pattern, providing intelligent and automated GC management for the system.

## 🎯 Go 1.26.0 优化 | Go 1.26.0 Optimizations

本包已针对 **Go 1.26.0** 进行全面优化，充分利用最新特性：

This package is fully optimized for **Go 1.26.0**, leveraging the latest features:

### 性能提升 | Performance Improvements

- **Green Tea GC**：自动受益于新的垃圾回收器，GC 开销降低 10-40% | Automatically benefits from the new garbage collector
  with 10-40% reduced GC overhead
- **Faster Allocations**：自动受益于小对象（1-512 字节）的专用分配优化 | Automatically benefits from specialized allocation
  optimizations for small objects (1-512 bytes)
- **Faster CGO/Syscalls**：系统调用开销降低约 30% | ~30% reduction in syscall overhead

### 新增监控能力 | New Monitoring Capabilities

- **Goroutine Metrics**：集成 `runtime/metrics` 新增的调度器指标 | Integrated with new scheduler metrics in
  `runtime/metrics`
    - `/sched/goroutines-created:goroutines` - 程序启动以来创建的 goroutine 总数 | Total goroutines created since
      program start
    - `/sched/goroutines/running:goroutines` - 正在执行的 goroutine 数量 | Currently running goroutines
    - `/sched/goroutines/runnable:goroutines` - 就绪但未执行的 goroutine 数量 | Runnable but not executing goroutines
    - `/sched/goroutines/waiting:goroutines` - 等待资源的 goroutine 数量 | Goroutines waiting on resources
    - `/sched/goroutines/not-in-go:goroutines` - 在系统调用/CGO中的 goroutine 数量 | Goroutines in syscall/CGO
    - `/sched/threads/total:threads` - 当前存活的线程数量 | Current live threads

- **Goroutine Leak Detection**：使用 `runtime/pprof` 的 `goroutineleak` profile 检测泄漏 | Uses `goroutineleak` profile
  in `runtime/pprof` for leak detection
    - 基于 GC 的泄漏检测机制 | GC-based leak detection mechanism
    - 自动识别长时间未活动的 goroutine | Automatically identifies long-inactive goroutines
    - 集成到安全性分析报告中 | Integrated into security analysis reports

### 代码优化 | Code Optimizations

- **new(expr) Syntax**：使用 Go 1.26 新语法优化代码 | Uses Go 1.26 new syntax for code optimization
    - 简化零值结构体初始化 | Simplified zero-value struct initialization
    - 提高代码可读性 | Improved code readability

## 🚀 功能特性 | Features

### 核心功能 | Core Features

- **定时GC执行 | Scheduled GC Execution**：支持配置定时间隔自动执行GC | Support configurable interval for automatic GC
  execution
- **智能GC策略 | Intelligent GC Strategies**：6种策略适应不同场景 | 6 strategies for different scenarios
- **内存阈值触发 | Memory Threshold Trigger**：可配置内存使用阈值自动触发GC | Configurable memory threshold to trigger GC
- **重试机制 | Retry Mechanism**：基于 `pkg/retry` 包的重试机制，确保GC执行的可靠性 | Based on `pkg/retry` package for
  reliable GC execution
- **统计信息收集 | Statistics Collection**：详细的GC执行统计信息 | Detailed GC execution statistics
- **优雅关闭 | Graceful Shutdown**：支持优雅启动和停止 | Support graceful start and stop
- **配置化管理 | Configuration Management**：通过配置文件灵活控制GC行为 | Flexible GC behavior control via configuration

### 高级功能 | Advanced Features

- **性能监控 | Performance Monitoring**：实时监控GC性能指标和趋势 | Real-time monitoring of GC performance metrics and
  trends
- **智能告警 | Intelligent Alerts**：基于阈值的自动告警系统 | Threshold-based automatic alert system
- **动态调优 | Dynamic Tuning**：自动调整GC策略和参数 | Automatic adjustment of GC strategies and parameters
- **单例架构 | Singleton Architecture**：线程安全的单例模式设计 | Thread-safe singleton pattern design
- **压力感知 | Pressure Awareness**：基于内存压力的智能GC调度 | Intelligent GC scheduling based on memory pressure
- **协程池集成 | Goroutine Pool Integration**：使用 `pkg/pool` 进行高效的协程管理 | Efficient goroutine management using
  `pkg/pool`
- **统一日志 | Unified Logging**：集成项目的 `pkg/logger` 进行统一日志管理 | Integrated with project's `pkg/logger` for
  unified logging

## 使用方法 | Usage

### 1. 配置文件 | Configuration File

在 `configs/config.yaml` 中添加GC配置 | Add GC configuration in `configs/config.yaml`:

```yaml
gc:
  enabled: true                    # 是否启用GC管理器 | Enable GC manager
  interval: 5m                     # 定时GC间隔时间（5分钟）| GC interval (5 minutes)
  memory_threshold: 104857600      # 内存阈值触发GC（100MB）| Memory threshold to trigger GC (100MB)
  strategy: standard               # GC策略 | GC strategy: standard, aggressive, gentle, adaptive, pressure_aware, scheduled
  force_gc: false                  # 是否强制执行完整GC | Force full GC
  free_os_memory: false            # 是否释放操作系统内存 | Free OS memory
  enable_stats: true               # 是否启用统计信息收集 | Enable statistics collection
  enable_monitoring: true          # 是否启用性能监控 | Enable performance monitoring
  enable_auto_tuning: false        # 是否启用自动调优 | Enable auto-tuning
  enable_alerts: true              # 是否启用GC告警 | Enable GC alerts
  max_retries: 3                   # 最大重试次数 | Max retry count
  retry_interval: 30s              # 重试间隔时间 | Retry interval
  execution_timeout: 60s           # GC执行超时时间 | GC execution timeout
  pressure_thresholds: # 内存压力阈值配置 | Memory pressure thresholds
    low: 52428800                  # 低压力阈值（50MB）| Low pressure threshold (50MB)
    medium: 104857600              # 中等压力阈值（100MB）| Medium pressure threshold (100MB)
    high: 209715200                # 高压力阈值（200MB）| High pressure threshold (200MB)
    critical: 524288000            # 临界压力阈值（500MB）| Critical pressure threshold (500MB)
```

### 2. 程序集成 | Program Integration

GC服务已经集成到主程序中，会在应用启动时自动启动，在应用关闭时自动停止。
The GC service is integrated into the main program and will start automatically when the application starts and stop
when it closes.

### 3. 手动使用 | Manual Usage

```go
package main

import (
	"fmt"
	"time"

	"go-port-forward/pkg/gc"
	"go-port-forward/pkg/logger"

	"go.uber.org/zap"
)

func main() {
	// 创建配置 | Create configuration
	config := gc.DefaultConfig()
	config.Interval = 1 * time.Minute
	config.Strategy = gc.StrategyStandard
	config.EnableMonitoring = true
	config.EnableStats = true

	// 创建GC服务（单例模式）| Create GC service (singleton pattern)
	service, err := gc.NewService(config)
	if err != nil {
		logger.Fatal("Failed to create GC service", zap.Error(err))
	}

	// 启动服务 | Start service
	if err = service.Start(); err != nil {
		logger.Fatal("Failed to start GC service", zap.Error(err))
	}

	// 手动触发GC | Manually trigger GC
	if err = service.ForceGC(); err != nil {
		logger.Error("Failed to force GC", zap.Error(err))
	}

	// 获取统计信息 | Get statistics
	stats := service.GetStats()
	fmt.Printf("Total runs: %d\n", stats.TotalRuns)
	fmt.Printf("Success count: %d\n", stats.SuccessCount)
	fmt.Printf("Failure count: %d\n", stats.FailureCount)

	// 获取健康状态 | Get health status
	health := service.GetHealth()
	fmt.Printf("Health status: %s\n", health.Status)

	// 停止服务 | Stop service
	if err = service.Stop(); err != nil {
		logger.Error("Failed to stop GC service", zap.Error(err))
	}
}
```

## 🎯 GC策略

### 1. Standard（标准策略）

- 执行 `runtime.GC()`
- 适用于大多数场景
- 性能影响较小

### 2. Aggressive（激进策略）

- 执行 `runtime.GC()` + `debug.FreeOSMemory()`
- 更彻底的内存清理
- 适用于内存敏感的环境

### 3. Gentle（温和策略）

- 只在堆内存使用超过阈值时执行GC
- 对性能影响最小
- 适用于性能敏感的场景

### 4. Adaptive（自适应策略）

- 根据内存增长速度动态调整GC行为
- 自动适应应用的内存使用模式
- 适用于内存使用波动较大的应用

### 5. Pressure-Aware（压力感知策略）

- 基于当前内存压力级别选择GC强度
- 四个压力级别：低、中、高、临界
- 在内存紧张时更激进，内存充足时更温和

### 6. Scheduled（时间调度策略）

- 基于时间调度执行不同强度的GC
- 默认调度：凌晨2点激进GC，其他时间标准/温和GC
- 适用于有明确业务时间窗口的应用

## 📊 统计信息

GC管理器提供详细的统计信息：

```go
type Stats struct {
TotalRuns          uint64 // 总执行次数
SuccessfulRuns     uint64 // 成功执行次数
FailedRuns         uint64 // 失败执行次数
LastRunTime        time.Time     // 最后执行时间
LastRunDuration    time.Duration // 最后执行耗时
AverageRunDuration time.Duration // 平均执行耗时
MemoryBeforeGC     uint64        // GC前内存使用量
MemoryAfterGC      uint64        // GC后内存使用量
MemoryFreed        uint64 // 释放的内存量
}
```

## 🔍 性能监控

启用性能监控后，系统会收集详细的性能指标：

```go
type PerformanceMetrics struct {
	// GC 性能指标 | GC performance metrics
	GCFrequency       float64       // 每分钟GC次数 | GC frequency per minute
	AvgGCDuration     time.Duration // 平均GC耗时 | Average GC duration
	MaxGCDuration     time.Duration // 最大GC耗时 | Max GC duration
	AvgMemoryFreed    uint64        // 平均释放内存 | Average memory freed
	MemoryEfficiency  float64       // 内存释放效率 | Memory free efficiency
	GCOverhead        float64       // GC开销占比 | GC overhead ratio
	MemoryGrowthRate  float64       // 内存增长率 | Memory growth rate
	GCPressureTrend   float64       // GC压力趋势 | GC pressure trend

	// Go 1.26+ goroutine 指标 | Go 1.26+ goroutine metrics
	GoroutinesCreated uint64 // 程序启动以来创建的 goroutine 总数 | Total goroutines created since program start
	GoroutinesLive    uint64 // 当前存活的 goroutine 数量 | Current live goroutines
	GoroutinesRunning uint64 // 正在执行的 goroutine 数量 | Currently running goroutines
	GoroutinesRunnable uint64 // 就绪但未执行的 goroutine 数量 | Runnable but not executing goroutines
	GoroutinesWaiting uint64 // 等待资源的 goroutine 数量 | Goroutines waiting on resources
	GoroutinesSyscall uint64 // 在系统调用/CGO中的 goroutine 数量 | Goroutines in syscall/CGO
	ThreadsLive       uint64 // 当前存活的线程数量 | Current live threads
	ThreadsMax        uint64 // GOMAXPROCS 设置的最大线程数 | Max threads (GOMAXPROCS)
}
```

### Go 1.26+ Goroutine 监控 | Go 1.26+ Goroutine Monitoring

性能报告现在包含详细的 goroutine 状态信息：

Performance reports now include detailed goroutine state information:

```
Go 1.26+ Goroutine Metrics:
---------------------------
Goroutines Created: 12345
Goroutines Live: 150
Goroutines Running: 8
Goroutines Runnable: 12
Goroutines Waiting: 120
Goroutines in Syscall: 10
Threads Live: 16
Threads Max (GOMAXPROCS): 16
```

这些指标可以帮助：

These metrics help with:

- **性能调优**：识别 goroutine 调度瓶颈 | **Performance Tuning**: Identify goroutine scheduling bottlenecks
- **资源监控**：跟踪 goroutine 和线程使用情况 | **Resource Monitoring**: Track goroutine and thread usage
- **泄漏检测**：发现异常的 goroutine 增长 | **Leak Detection**: Discover abnormal goroutine growth

### 智能告警

#### 告警配置

可以通过配置启用或禁用GC告警功能：

```go
config := &gc.Config{
Enabled:          true,
EnableMonitoring: true, // 必须启用监控才能使用告警
EnableAlerts:     true, // 启用/禁用告警
// ... 其他配置
}
```

#### 告警触发条件

当启用告警时，系统会在以下情况自动发出告警：

- GC耗时超过100ms
- 内存释放量低于1MB
- GC频率超过10次/分钟
- 内存释放效率低于10%
- GC开销超过5%

#### 自定义告警阈值

```go
// 获取性能监控器
monitor := service.GetPerformanceMonitor()

// 自定义告警阈值
customThresholds := &gc.AlertThresholds{
MaxGCDuration:  50 * time.Millisecond, // 更严格的GC耗时阈值
MinMemoryFreed: 5 * 1024 * 1024, // 最少释放5MB内存
MaxGCFrequency: 5.0,                   // 每分钟最多5次GC
MinEfficiency:  0.2,                   // 最少释放20%内存
MaxOverhead:    0.03, // 最多3%开销
}

// 更新告警阈值
monitor.UpdateThresholds(customThresholds)
```

#### 动态控制告警

```go
// 运行时禁用告警
newConfig := *currentConfig
newConfig.EnableAlerts = false
service.ReloadConfig(&newConfig)

// 运行时启用告警
newConfig.EnableAlerts = true
service.ReloadConfig(&newConfig)
```

## 🔧 动态调优

启用自动调优后，系统会根据性能指标自动调整：

### 调优规则

1. **高频率GC** → 切换到温和策略
2. **低内存效率** → 切换到激进策略
3. **长GC耗时** → 增加GC间隔
4. **高内存增长率** → 减少GC间隔

### 手动调优

```go
// 获取调优器
tuner := service.GetTuner()

// 添加自定义规则
rule := TuningRule{
Name: "Custom Rule",
Condition: TuningCondition{
MetricType: "gc_frequency",
Operator: ">",
Threshold: 5.0,
},
Action: TuningAction{
Type: "change_strategy",
Parameters: map[string]interface{}{
"strategy": "gentle",
},
},
}
tuner.AddTuningRule(rule)
```

## ⚙️ 配置说明

| 配置项                  | 类型     | 默认值        | 说明             |
|----------------------|--------|------------|----------------|
| `enabled`            | bool   | true       | 是否启用GC管理器      |
| `interval`           | string | "5m"       | 定时GC间隔时间       |
| `memory_threshold`   | uint64 | 104857600  | 内存阈值（字节），0表示禁用 |
| `strategy`           | string | "standard" | GC策略类型（6种可选）   |
| `force_gc`           | bool   | false      | 是否强制执行完整GC     |
| `free_os_memory`     | bool   | false      | 是否释放操作系统内存     |
| `enable_stats`       | bool   | true       | 是否启用统计信息收集     |
| `enable_monitoring`  | bool   | true       | 是否启用性能监控       |
| `enable_auto_tuning` | bool   | false      | 是否启用自动调优       |
| `enable_alerts`      | bool   | true       | 是否启用GC性能告警     |
| `max_retries`        | int    | 3          | 最大重试次数         |
| `retry_interval`     | string | "30s"      | 重试间隔时间         |

### 压力阈值配置

| 配置项                            | 类型     | 默认值       | 说明            |
|--------------------------------|--------|-----------|---------------|
| `pressure_thresholds.low`      | uint64 | 52428800  | 低压力阈值（50MB）   |
| `pressure_thresholds.medium`   | uint64 | 104857600 | 中等压力阈值（100MB） |
| `pressure_thresholds.high`     | uint64 | 209715200 | 高压力阈值（200MB）  |
| `pressure_thresholds.critical` | uint64 | 524288000 | 临界压力阈值（500MB） |

## 🎯 使用场景推荐

### 开发环境

```yaml
gc:
  strategy: adaptive
  interval: 10m              # 较长间隔，减少干扰
  enable_monitoring: true
  enable_auto_tuning: true
  enable_alerts: false       # 开发环境建议禁用告警
```

### 测试环境

```yaml
gc:
  strategy: pressure_aware
  interval: 5m
  enable_monitoring: true
  enable_auto_tuning: true
  enable_alerts: true        # 测试环境启用告警，便于发现问题
```

### 生产环境

```yaml
gc:
  strategy: scheduled
  interval: 3m               # 较短间隔，保证性能
  enable_monitoring: true
  enable_auto_tuning: false  # 生产环境建议手动调优
  enable_alerts: true        # 生产环境必须启用告警
  memory_threshold: 52428800 # 更低的内存阈值（50MB）
```

## 🧪 测试

### 基础测试

```bash
# 运行所有测试
go test ./pkg/gc -v

# 运行示例
go test ./pkg/gc -run Example

# 运行数据竞争检测
go test ./pkg/gc -v -race

# 运行单例测试
go test ./pkg/gc -v -run "TestSingleton"

# 运行性能测试
go test ./pkg/gc -v -run "TestEnhanced"
```

### 基准测试

```bash
# 运行基准测试
go test ./pkg/gc -bench=.

# 运行单例创建基准测试
go test ./pkg/gc -bench=BenchmarkSingletonCreation
```

## ⚠️ 注意事项

1. **性能影响**：GC操作会暂停程序执行，建议根据应用特性调整执行频率
2. **内存阈值**：设置合理的内存阈值，避免频繁触发GC
3. **策略选择**：根据应用场景选择合适的GC策略
4. **监控统计**：启用统计信息收集，便于监控GC效果
5. **优雅关闭**：应用关闭时会自动停止GC服务
6. **单例模式**：所有组件都是线程安全的单例，避免重复创建
7. **自动调优**：生产环境建议关闭自动调优，使用手动调优
8. **压力阈值**：根据应用内存使用情况调整压力阈值

## 📈 监控指标

建议监控以下关键指标：

- GC执行频率和耗时
- 内存释放效果
- 错误率和重试次数
- 内存增长趋势
- GC压力变化

## 🚨 告警设置

建议设置以下告警：

- GC失败率 > 5%
- 平均执行时间 > 10s
- 内存释放效果 < 预期
- GC频率异常高/低

## 📦 依赖

- `pkg/retry`：重试机制
- `go.uber.org/zap`：结构化日志

## 🎉 总结

GC管理器是一个**企业级、生产就绪**的内存管理解决方案，提供：

- ✅ **智能化**：6种策略自动适应不同场景
- ✅ **可观测**：全面的性能监控和告警
- ✅ **自动化**：动态调优减少人工干预
- ✅ **高可靠**：单例模式确保线程安全
- ✅ **易扩展**：模块化设计便于后续扩展

为系统的长期稳定运行提供强有力的保障！
