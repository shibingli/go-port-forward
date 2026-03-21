// Package gc 提供内存压力感知的GC优化 | Provides memory pressure-aware GC optimization
package gc

import (
	"context"
	"runtime"
	"runtime/debug"
	"sync"
	"time"

	"go-port-forward/pkg/logger"

	"go.uber.org/zap"
)

// 全局单例实例
var (
	globalPressureAwareStrategy *PressureAwareStrategy
	globalScheduledStrategy     *ScheduledStrategy
	pressureStrategyOnce        sync.Once
	scheduledStrategyOnce       sync.Once
)

// MemoryPressure 内存压力级别 | Memory pressure level
type MemoryPressure int

const (
	PressureLow MemoryPressure = iota
	PressureMedium
	PressureHigh
	PressureCritical
)

// PressureAwareStrategy 内存压力感知GC策略 | Memory pressure-aware GC strategy
type PressureAwareStrategy struct {
	lastPressureCheck time.Time

	// 配置参数
	lowThreshold      uint64 // 低压力阈值
	mediumThreshold   uint64 // 中等压力阈值
	highThreshold     uint64 // 高压力阈值
	criticalThreshold uint64 // 临界压力阈值

	mu sync.RWMutex

	currentPressure MemoryPressure
}

// NewPressureAwareStrategy 获取全局单例内存压力感知策略 | Get global singleton pressure-aware strategy
func NewPressureAwareStrategy() *PressureAwareStrategy {
	pressureStrategyOnce.Do(func() {
		logger.Info("Initializing global Pressure Aware Strategy singleton")

		globalPressureAwareStrategy = &PressureAwareStrategy{
			lowThreshold:      50 * 1024 * 1024,  // 50MB
			mediumThreshold:   100 * 1024 * 1024, // 100MB
			highThreshold:     200 * 1024 * 1024, // 200MB
			criticalThreshold: 500 * 1024 * 1024, // 500MB
		}

		logger.Info("Global Pressure Aware Strategy initialized successfully")
	})

	return globalPressureAwareStrategy
}

// Execute 执行压力感知GC | Execute pressure-aware GC
func (s *PressureAwareStrategy) Execute(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		pressure := s.assessMemoryPressure()
		return s.executeBasedOnPressure(pressure)
	}
}

// Name 返回策略名称 | Return strategy name
func (s *PressureAwareStrategy) Name() string {
	return "pressure_aware"
}

// assessMemoryPressure 评估当前内存压力
func (s *PressureAwareStrategy) assessMemoryPressure() MemoryPressure {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()

	// 限制压力检查频率，避免过于频繁
	if now.Sub(s.lastPressureCheck) < 5*time.Second {
		return s.currentPressure
	}

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	heapInUse := m.HeapInuse

	var pressure MemoryPressure
	switch {
	case heapInUse >= s.criticalThreshold:
		pressure = PressureCritical
	case heapInUse >= s.highThreshold:
		pressure = PressureHigh
	case heapInUse >= s.mediumThreshold:
		pressure = PressureMedium
	default:
		pressure = PressureLow
	}

	s.currentPressure = pressure
	s.lastPressureCheck = now

	logger.Debug("Memory pressure assessed",
		zap.String("pressure", s.pressureToString(pressure)),
		zap.Uint64("heap_inuse", heapInUse),
		zap.Uint64("heap_alloc", m.HeapAlloc))

	return pressure
}

// executeBasedOnPressure 根据内存压力执行相应的GC策略
func (s *PressureAwareStrategy) executeBasedOnPressure(pressure MemoryPressure) error {
	switch pressure {
	case PressureLow:
		// 低压力：跳过GC或执行轻量级GC
		return s.executeLightGC()

	case PressureMedium:
		// 中等压力：标准GC
		runtime.GC()
		return nil

	case PressureHigh:
		// 高压力：激进GC
		runtime.GC()
		debug.FreeOSMemory()
		return nil

	case PressureCritical:
		// 临界压力：强制GC + 内存整理
		return s.executeCriticalGC()

	default:
		runtime.GC()
		return nil
	}
}

// executeLightGC 执行轻量级GC
func (s *PressureAwareStrategy) executeLightGC() error {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// 只有在确实需要时才执行GC
	if m.HeapAlloc > s.lowThreshold {
		runtime.GC()
	}

	return nil
}

// executeCriticalGC 执行临界状态GC
func (s *PressureAwareStrategy) executeCriticalGC() error {
	logger.Warn("Critical memory pressure detected, executing emergency GC")

	// 多次GC确保彻底清理
	for i := 0; i < 3; i++ {
		runtime.GC()
	}

	// 强制释放操作系统内存
	debug.FreeOSMemory()

	// 设置更激进的GC目标
	oldGCPercent := debug.SetGCPercent(50) // 临时降低GC阈值
	defer debug.SetGCPercent(oldGCPercent)

	runtime.GC()

	return nil
}

// pressureToString 将压力级别转换为字符串
func (s *PressureAwareStrategy) pressureToString(pressure MemoryPressure) string {
	switch pressure {
	case PressureLow:
		return "low"
	case PressureMedium:
		return "medium"
	case PressureHigh:
		return "high"
	case PressureCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// GetCurrentPressure 获取当前内存压力级别 | Get current memory pressure level
func (s *PressureAwareStrategy) GetCurrentPressure() MemoryPressure {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentPressure
}

// UpdateThresholds 更新压力阈值 | Update pressure thresholds
func (s *PressureAwareStrategy) UpdateThresholds(low, medium, high, critical uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.lowThreshold = low
	s.mediumThreshold = medium
	s.highThreshold = high
	s.criticalThreshold = critical

	logger.Info("Memory pressure thresholds updated",
		zap.Uint64("low", low),
		zap.Uint64("medium", medium),
		zap.Uint64("high", high),
		zap.Uint64("critical", critical))
}

// ScheduledStrategy 基于时间调度的GC策略 | Time-scheduled GC strategy
type ScheduledStrategy struct {
	lastExecution time.Time
	schedule      []ScheduleEntry
	mu            sync.RWMutex
}

// ScheduleEntry 调度条目 | Schedule entry
type ScheduleEntry struct {
	Strategy StrategyType // 使用的策略 | Strategy to use

	Hour   int  // 小时 (0-23) | Hour (0-23)
	Minute int  // 分钟 (0-59) | Minute (0-59)
	Force  bool // 是否强制执行 | Whether to force execution
}

// NewScheduledStrategy 获取全局单例基于时间调度的策略 | Get global singleton scheduled strategy
func NewScheduledStrategy() *ScheduledStrategy {
	scheduledStrategyOnce.Do(func() {
		logger.Info("Initializing global Scheduled Strategy singleton")

		// 默认调度：凌晨2点执行激进GC，其他时间执行标准GC
		defaultSchedule := []ScheduleEntry{
			{Hour: 2, Minute: 0, Strategy: StrategyAggressive, Force: true}, // 凌晨2点激进GC
			{Hour: 8, Minute: 0, Strategy: StrategyStandard, Force: false},  // 上午8点标准GC
			{Hour: 14, Minute: 0, Strategy: StrategyStandard, Force: false}, // 下午2点标准GC
			{Hour: 20, Minute: 0, Strategy: StrategyGentle, Force: false},   // 晚上8点温和GC
		}

		globalScheduledStrategy = &ScheduledStrategy{
			schedule: defaultSchedule,
		}

		logger.Info("Global Scheduled Strategy initialized successfully")
	})

	return globalScheduledStrategy
}

// resetStrategiesForTesting 重置策略单例（仅用于测试）
func resetStrategiesForTesting() {
	globalPressureAwareStrategy = nil
	globalScheduledStrategy = nil
	pressureStrategyOnce = sync.Once{}
	scheduledStrategyOnce = sync.Once{}
}

// Execute 执行调度策略 | Execute scheduled strategy
func (s *ScheduledStrategy) Execute(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return s.executeScheduled()
	}
}

// Name 返回策略名称 | Return strategy name
func (s *ScheduledStrategy) Name() string {
	return "scheduled"
}

// executeScheduled 执行调度的GC
func (s *ScheduledStrategy) executeScheduled() error {
	now := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()

	// 检查是否有匹配的调度条目
	for _, entry := range s.schedule {
		if s.shouldExecute(now, entry) {
			s.lastExecution = now
			return s.executeByStrategy(entry.Strategy, entry.Force)
		}
	}

	// 没有匹配的调度，执行默认策略
	return s.executeByStrategy(StrategyStandard, false)
}

// shouldExecute 检查是否应该执行指定的调度条目
func (s *ScheduledStrategy) shouldExecute(now time.Time, entry ScheduleEntry) bool {
	// 检查时间是否匹配
	if now.Hour() != entry.Hour || now.Minute() != entry.Minute {
		return false
	}

	// 避免重复执行（同一分钟内）
	if !s.lastExecution.IsZero() &&
		now.Sub(s.lastExecution) < time.Minute {
		return false
	}

	return true
}

// executeByStrategy 根据策略类型执行GC
func (s *ScheduledStrategy) executeByStrategy(strategyType StrategyType, force bool) error {
	switch strategyType {
	case StrategyStandard:
		runtime.GC()
	case StrategyAggressive:
		runtime.GC()
		debug.FreeOSMemory()
	case StrategyGentle:
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		if force || m.HeapAlloc > 50*1024*1024 {
			runtime.GC()
		}
	default:
		// 未知策略类型，使用标准GC作为默认策略
		logger.Warn("Unknown strategy type, using standard GC",
			zap.String("strategy", string(strategyType)))
		runtime.GC()
	}

	return nil
}

// AddScheduleEntry 添加调度条目 | Add schedule entry
func (s *ScheduledStrategy) AddScheduleEntry(entry ScheduleEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.schedule = append(s.schedule, entry)

	logger.Info("Schedule entry added",
		zap.Int("hour", entry.Hour),
		zap.Int("minute", entry.Minute),
		zap.String("strategy", string(entry.Strategy)),
		zap.Bool("force", entry.Force))
}
