// Package gc 提供GC性能监控和调优功能 | Provides GC performance monitoring and tuning
package gc

import (
	"fmt"
	"runtime"
	"runtime/metrics"
	"sync"
	"time"

	"go-port-forward/pkg/logger"

	"go.uber.org/zap"
)

// 全局单例实例
var (
	globalPerformanceMonitor *PerformanceMonitor
	monitorOnce              sync.Once
)

// PerformanceMonitor GC性能监控器 | GC performance monitor
type PerformanceMonitor struct {
	lastAlert       time.Time
	metrics         *PerformanceMetrics
	alertThresholds *AlertThresholds
	config          *Config
	history         []PerformanceSnapshot
	alertCooldown   time.Duration
	maxHistorySize  int
	mu              sync.RWMutex
}

// PerformanceMetrics GC性能指标 | GC performance metrics
type PerformanceMetrics struct {
	AvgGCDuration time.Duration // 平均GC耗时 | Average GC duration
	MaxGCDuration time.Duration // 最大GC耗时 | Max GC duration
	MinGCDuration time.Duration // 最小GC耗时 | Min GC duration

	AvgMemoryFreed uint64 // 平均释放内存 | Average memory freed

	MemoryEfficiency float64 // 内存释放效率 (释放量/GC前内存) | Memory free efficiency
	GCOverhead       float64 // GC开销占比 | GC overhead ratio
	ThroughputImpact float64 // 对吞吐量的影响 | Throughput impact
	MemoryGrowthRate float64 // 内存增长率 | Memory growth rate
	GCPressureTrend  float64 // GC压力趋势 | GC pressure trend
	GCFrequency      float64 // 每分钟GC次数 | GC frequency per minute

	// Go 1.26+ goroutine metrics | Go 1.26+ goroutine 指标
	GoroutinesCreated  uint64 // 程序启动以来创建的 goroutine 总数 | Total goroutines created since program start
	GoroutinesLive     uint64 // 当前存活的 goroutine 数量 | Current live goroutines
	GoroutinesRunning  uint64 // 正在执行的 goroutine 数量 | Currently running goroutines
	GoroutinesRunnable uint64 // 就绪但未执行的 goroutine 数量 | Runnable but not executing goroutines
	GoroutinesWaiting  uint64 // 等待资源的 goroutine 数量 | Goroutines waiting on resources
	GoroutinesSyscall  uint64 // 在系统调用/CGO中的 goroutine 数量 | Goroutines in syscall/CGO
	ThreadsLive        uint64 // 当前存活的线程数量 | Current live threads
	ThreadsMax         uint64 // GOMAXPROCS 设置的最大线程数 | Max threads (GOMAXPROCS)
}

// PerformanceSnapshot 性能快照 | Performance snapshot
type PerformanceSnapshot struct {
	Timestamp  time.Time
	GCDuration time.Duration

	NumGoroutines int

	MemoryBefore uint64
	MemoryAfter  uint64
	MemoryFreed  uint64
	HeapSize     uint64

	GCCycles uint32

	// Go 1.26+ goroutine 详细状态 | Go 1.26+ detailed goroutine states
	GoroutinesRunning  uint64 // 正在执行的 goroutine | Running goroutines
	GoroutinesRunnable uint64 // 就绪的 goroutine | Runnable goroutines
	GoroutinesWaiting  uint64 // 等待的 goroutine | Waiting goroutines
	GoroutinesSyscall  uint64 // 系统调用中的 goroutine | Goroutines in syscall
	ThreadsLive        uint64 // 存活的线程数 | Live threads
}

// AlertThresholds 告警阈值 | Alert thresholds
type AlertThresholds struct {
	MaxGCDuration  time.Duration // 最大GC耗时阈值 | Max GC duration threshold
	MinMemoryFreed uint64        // 最小内存释放阈值 | Min memory freed threshold
	MaxGCFrequency float64       // 最大GC频率阈值 | Max GC frequency threshold
	MinEfficiency  float64       // 最小效率阈值 | Min efficiency threshold
	MaxOverhead    float64       // 最大开销阈值 | Max overhead threshold
}

// NewPerformanceMonitor 获取全局单例性能监控器 | Get global singleton performance monitor
func NewPerformanceMonitor() *PerformanceMonitor {
	monitorOnce.Do(func() {
		logger.Info("Initializing global Performance Monitor singleton")

		globalPerformanceMonitor = &PerformanceMonitor{
			metrics:        new(PerformanceMetrics),
			history:        make([]PerformanceSnapshot, 0),
			maxHistorySize: 1000, // 保留最近1000次记录
			alertThresholds: &AlertThresholds{
				MaxGCDuration:  100 * time.Millisecond,
				MinMemoryFreed: 1024 * 1024, // 1MB
				MaxGCFrequency: 10.0,        // 每分钟最多10次
				MinEfficiency:  0.1,         // 最少释放10%内存
				MaxOverhead:    0.05,        // 最多5%开销
			},
			alertCooldown: 5 * time.Minute, // 告警冷却时间
		}

		logger.Info("Global Performance Monitor initialized successfully")
	})

	return globalPerformanceMonitor
}

// GetGlobalPerformanceMonitor 直接获取全局性能监控器实例 | Get global performance monitor instance directly
func GetGlobalPerformanceMonitor() *PerformanceMonitor {
	return globalPerformanceMonitor
}

// resetMonitorForTesting 重置监控器单例（仅用于测试）
func resetMonitorForTesting() {
	globalPerformanceMonitor = nil
	monitorOnce = sync.Once{}
}

// RecordGCExecution 记录GC执行 | Record GC execution
func (pm *PerformanceMonitor) RecordGCExecution(duration time.Duration, memBefore, memAfter uint64) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	var memoryFreed uint64
	if memBefore > memAfter {
		memoryFreed = memBefore - memAfter
	} else {
		memoryFreed = 0
	}

	// 收集 Go 1.26+ goroutine 指标 | Collect Go 1.26+ goroutine metrics
	goroutineMetrics := pm.collectGoroutineMetrics()

	snapshot := PerformanceSnapshot{
		Timestamp:     time.Now(),
		GCDuration:    duration,
		MemoryBefore:  memBefore,
		MemoryAfter:   memAfter,
		MemoryFreed:   memoryFreed,
		HeapSize:      pm.getCurrentHeapSize(),
		NumGoroutines: runtime.NumGoroutine(),
		GCCycles:      pm.getCurrentGCCycles(),

		// Go 1.26+ goroutine 状态 | Go 1.26+ goroutine states
		GoroutinesRunning:  goroutineMetrics.running,
		GoroutinesRunnable: goroutineMetrics.runnable,
		GoroutinesWaiting:  goroutineMetrics.waiting,
		GoroutinesSyscall:  goroutineMetrics.syscall,
		ThreadsLive:        goroutineMetrics.threadsLive,
	}

	// 添加到历史记录
	pm.addSnapshot(snapshot)

	// 更新性能指标
	pm.updateMetrics()

	// 检查告警条件
	pm.checkAlerts(snapshot)
}

// addSnapshot 添加性能快照
func (pm *PerformanceMonitor) addSnapshot(snapshot PerformanceSnapshot) {
	pm.history = append(pm.history, snapshot)

	// 限制历史记录大小
	if len(pm.history) > pm.maxHistorySize {
		pm.history = pm.history[1:]
	}
}

// updateMetrics 更新性能指标
func (pm *PerformanceMonitor) updateMetrics() {
	if len(pm.history) == 0 {
		return
	}

	// 计算时间窗口（最近5分钟）
	now := time.Now()
	windowStart := now.Add(-5 * time.Minute)

	var recentSnapshots []PerformanceSnapshot
	for _, snapshot := range pm.history {
		if snapshot.Timestamp.After(windowStart) {
			recentSnapshots = append(recentSnapshots, snapshot)
		}
	}

	if len(recentSnapshots) == 0 {
		return
	}

	// 计算GC频率
	timeSpan := now.Sub(recentSnapshots[0].Timestamp)
	if timeSpan > 0 {
		pm.metrics.GCFrequency = float64(len(recentSnapshots)) / timeSpan.Minutes()
	}

	// 计算平均、最大、最小GC耗时
	var totalDuration time.Duration
	maxDuration := time.Duration(0)
	minDuration := time.Duration(1<<63 - 1) // 最大值

	var totalMemoryFreed uint64
	var totalMemoryBefore uint64

	for _, snapshot := range recentSnapshots {
		totalDuration += snapshot.GCDuration
		totalMemoryFreed += snapshot.MemoryFreed
		totalMemoryBefore += snapshot.MemoryBefore

		if snapshot.GCDuration > maxDuration {
			maxDuration = snapshot.GCDuration
		}
		if snapshot.GCDuration < minDuration {
			minDuration = snapshot.GCDuration
		}
	}

	count := len(recentSnapshots)
	pm.metrics.AvgGCDuration = totalDuration / time.Duration(count)
	pm.metrics.MaxGCDuration = maxDuration
	pm.metrics.MinGCDuration = minDuration
	pm.metrics.AvgMemoryFreed = totalMemoryFreed / uint64(count)

	// 计算内存释放效率
	if totalMemoryBefore > 0 {
		pm.metrics.MemoryEfficiency = float64(totalMemoryFreed) / float64(totalMemoryBefore)
	}

	// 计算GC开销
	totalGCTime := totalDuration
	if timeSpan > 0 {
		pm.metrics.GCOverhead = totalGCTime.Seconds() / timeSpan.Seconds()
	}

	// 聚合 Go 1.26+ goroutine 指标 | Aggregate Go 1.26+ goroutine metrics
	var totalRunning, totalRunnable, totalWaiting, totalSyscall, totalThreads uint64
	for _, snapshot := range recentSnapshots {
		totalRunning += snapshot.GoroutinesRunning
		totalRunnable += snapshot.GoroutinesRunnable
		totalWaiting += snapshot.GoroutinesWaiting
		totalSyscall += snapshot.GoroutinesSyscall
		totalThreads += snapshot.ThreadsLive
	}
	pm.metrics.GoroutinesRunning = totalRunning / uint64(count)
	pm.metrics.GoroutinesRunnable = totalRunnable / uint64(count)
	pm.metrics.GoroutinesWaiting = totalWaiting / uint64(count)
	pm.metrics.GoroutinesSyscall = totalSyscall / uint64(count)
	pm.metrics.ThreadsLive = totalThreads / uint64(count)

	// 获取累计创建的 goroutine 数和当前存活数 | Get total created and live goroutines
	samples := []metrics.Sample{
		{Name: "/sched/goroutines-created:goroutines"},
	}
	metrics.Read(samples)
	pm.metrics.GoroutinesCreated = samples[0].Value.Uint64()
	pm.metrics.GoroutinesLive = uint64(runtime.NumGoroutine())
	pm.metrics.ThreadsMax = uint64(runtime.GOMAXPROCS(0))

	// 计算内存增长率和GC压力趋势
	pm.calculateTrends(recentSnapshots)
}

// calculateTrends 计算趋势指标
func (pm *PerformanceMonitor) calculateTrends(snapshots []PerformanceSnapshot) {
	if len(snapshots) < 2 {
		return
	}

	// 计算内存增长率
	first := snapshots[0]
	last := snapshots[len(snapshots)-1]

	timeSpan := last.Timestamp.Sub(first.Timestamp)
	if timeSpan > 0 {
		memoryChange := int64(last.HeapSize) - int64(first.HeapSize)
		pm.metrics.MemoryGrowthRate = float64(memoryChange) / timeSpan.Hours() // 每小时增长
	}

	// 计算GC压力趋势（基于GC频率变化）
	if len(snapshots) >= 10 {
		mid := len(snapshots) / 2
		firstHalf := snapshots[:mid]
		secondHalf := snapshots[mid:]

		firstHalfFreq := float64(len(firstHalf)) / snapshots[mid-1].Timestamp.Sub(snapshots[0].Timestamp).Minutes()
		secondHalfFreq := float64(len(secondHalf)) / snapshots[len(snapshots)-1].Timestamp.Sub(snapshots[mid].Timestamp).Minutes()

		pm.metrics.GCPressureTrend = secondHalfFreq - firstHalfFreq
	}
}

// checkAlerts 检查告警条件
func (pm *PerformanceMonitor) checkAlerts(snapshot PerformanceSnapshot) {
	// 检查告警是否启用
	if pm.config != nil && !pm.config.IsAlertsEnabled() {
		return
	}

	now := time.Now()

	// 检查告警冷却时间
	if now.Sub(pm.lastAlert) < pm.alertCooldown {
		return
	}

	alerts := make([]string, 0)

	// 检查GC耗时
	if snapshot.GCDuration > pm.alertThresholds.MaxGCDuration {
		alerts = append(alerts, "GC duration exceeded threshold")
	}

	// 检查内存释放量
	if snapshot.MemoryFreed < pm.alertThresholds.MinMemoryFreed {
		alerts = append(alerts, "Memory freed below threshold")
	}

	// 检查GC频率
	if pm.metrics.GCFrequency > pm.alertThresholds.MaxGCFrequency {
		alerts = append(alerts, "GC frequency too high")
	}

	// 检查效率
	if pm.metrics.MemoryEfficiency < pm.alertThresholds.MinEfficiency {
		alerts = append(alerts, "GC efficiency too low")
	}

	// 检查开销
	if pm.metrics.GCOverhead > pm.alertThresholds.MaxOverhead {
		alerts = append(alerts, "GC overhead too high")
	}

	// 发送告警
	if len(alerts) > 0 {
		pm.sendAlerts(alerts, snapshot)
		pm.lastAlert = now
	}
}

// sendAlerts 发送告警
func (pm *PerformanceMonitor) sendAlerts(alerts []string, snapshot PerformanceSnapshot) {
	for _, alert := range alerts {
		logger.Warn("GC Performance Alert",
			zap.String("alert", alert),
			zap.Duration("gc_duration", snapshot.GCDuration),
			zap.Uint64("memory_freed", snapshot.MemoryFreed),
			zap.Float64("gc_frequency", pm.metrics.GCFrequency),
			zap.Float64("efficiency", pm.metrics.MemoryEfficiency),
			zap.Float64("overhead", pm.metrics.GCOverhead))
	}
}

// GetMetrics 获取当前性能指标 | Get current performance metrics
func (pm *PerformanceMonitor) GetMetrics() PerformanceMetrics {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	return *pm.metrics
}

// GetRecentHistory 获取最近的历史记录 | Get recent history
func (pm *PerformanceMonitor) GetRecentHistory(duration time.Duration) []PerformanceSnapshot {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	cutoff := time.Now().Add(-duration)
	var recent []PerformanceSnapshot

	for _, snapshot := range pm.history {
		if snapshot.Timestamp.After(cutoff) {
			recent = append(recent, snapshot)
		}
	}

	return recent
}

// getCurrentHeapSize 获取当前堆大小
func (pm *PerformanceMonitor) getCurrentHeapSize() uint64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.HeapInuse
}

// getCurrentGCCycles 获取当前GC周期数
func (pm *PerformanceMonitor) getCurrentGCCycles() uint32 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.NumGC
}

// goroutineMetrics Go 1.26+ goroutine 指标结构 | Go 1.26+ goroutine metrics structure
type goroutineMetrics struct {
	running     uint64 // 正在执行的 goroutine | Running goroutines
	runnable    uint64 // 就绪的 goroutine | Runnable goroutines
	waiting     uint64 // 等待的 goroutine | Waiting goroutines
	syscall     uint64 // 系统调用中的 goroutine | Goroutines in syscall
	threadsLive uint64 // 存活的线程数 | Live threads
}

// collectGoroutineMetrics 收集 Go 1.26+ goroutine 指标 | Collect Go 1.26+ goroutine metrics
func (pm *PerformanceMonitor) collectGoroutineMetrics() goroutineMetrics {
	// 定义需要读取的指标 | Define metrics to read
	samples := []metrics.Sample{
		{Name: "/sched/goroutines/running:goroutines"},
		{Name: "/sched/goroutines/runnable:goroutines"},
		{Name: "/sched/goroutines/waiting:goroutines"},
		{Name: "/sched/goroutines/not-in-go:goroutines"}, // syscall/CGO
		{Name: "/sched/threads/total:threads"},
	}

	// 读取指标 | Read metrics
	metrics.Read(samples)

	// 提取指标值 | Extract metric values
	result := goroutineMetrics{
		running:     samples[0].Value.Uint64(),
		runnable:    samples[1].Value.Uint64(),
		waiting:     samples[2].Value.Uint64(),
		syscall:     samples[3].Value.Uint64(),
		threadsLive: samples[4].Value.Uint64(),
	}

	return result
}

// UpdateThresholds 更新告警阈值 | Update alert thresholds
func (pm *PerformanceMonitor) UpdateThresholds(thresholds *AlertThresholds) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.alertThresholds = thresholds

	logger.Info("GC alert thresholds updated",
		zap.Duration("max_gc_duration", thresholds.MaxGCDuration),
		zap.Uint64("min_memory_freed", thresholds.MinMemoryFreed),
		zap.Float64("max_gc_frequency", thresholds.MaxGCFrequency),
		zap.Float64("min_efficiency", thresholds.MinEfficiency),
		zap.Float64("max_overhead", thresholds.MaxOverhead))
}

// SetConfig 设置GC配置 | Set GC configuration
func (pm *PerformanceMonitor) SetConfig(config *Config) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.config = config

	logger.Info("GC monitor config updated",
		zap.Bool("alerts_enabled", config != nil && config.IsAlertsEnabled()))
}

// GenerateReport 生成性能报告 | Generate performance report
func (pm *PerformanceMonitor) GenerateReport() string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	metrics := pm.metrics

	report := fmt.Sprintf("GC Performance Report:\n")
	report += fmt.Sprintf("======================\n")
	report += fmt.Sprintf("GC Frequency: %.2f/min\n", metrics.GCFrequency)
	report += fmt.Sprintf("Avg GC Duration: %v\n", metrics.AvgGCDuration)
	report += fmt.Sprintf("Max GC Duration: %v\n", metrics.MaxGCDuration)
	report += fmt.Sprintf("Avg Memory Freed: %d bytes\n", metrics.AvgMemoryFreed)
	report += fmt.Sprintf("Memory Efficiency: %.2f%%\n", metrics.MemoryEfficiency*100)
	report += fmt.Sprintf("GC Overhead: %.2f%%\n", metrics.GCOverhead*100)
	report += fmt.Sprintf("Memory Growth Rate: %.2f bytes/hour\n", metrics.MemoryGrowthRate)
	report += fmt.Sprintf("GC Pressure Trend: %.2f\n", metrics.GCPressureTrend)
	report += fmt.Sprintf("\n")
	report += fmt.Sprintf("Go 1.26+ Goroutine Metrics:\n")
	report += fmt.Sprintf("---------------------------\n")
	report += fmt.Sprintf("Goroutines Created: %d\n", metrics.GoroutinesCreated)
	report += fmt.Sprintf("Goroutines Live: %d\n", metrics.GoroutinesLive)
	report += fmt.Sprintf("Goroutines Running: %d\n", metrics.GoroutinesRunning)
	report += fmt.Sprintf("Goroutines Runnable: %d\n", metrics.GoroutinesRunnable)
	report += fmt.Sprintf("Goroutines Waiting: %d\n", metrics.GoroutinesWaiting)
	report += fmt.Sprintf("Goroutines in Syscall: %d\n", metrics.GoroutinesSyscall)
	report += fmt.Sprintf("Threads Live: %d\n", metrics.ThreadsLive)
	report += fmt.Sprintf("Threads Max (GOMAXPROCS): %d\n", metrics.ThreadsMax)

	return report
}
