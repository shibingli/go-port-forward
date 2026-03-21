// Package gc 提供GC管理器的安全性和性能分析工具 | Provides GC manager safety and performance analysis tools
package gc

import (
	"context"
	"fmt"
	"runtime"
	"runtime/pprof"
	"sync"
	"sync/atomic"
	"time"

	"go-port-forward/pkg/logger"

	"go.uber.org/zap"
)

// AnalysisReport 分析报告 | Analysis report
type AnalysisReport struct {
	// 分析时间 | Analysis time
	AnalysisTime time.Time `json:"analysis_time" msgpack:"analysis_time"`

	// 数据竞争检查 | Data race issues
	DataRaceIssues []string `json:"data_race_issues" msgpack:"data_race_issues"`

	// 性能问题 | Performance issues
	PerformanceIssues []string `json:"performance_issues" msgpack:"performance_issues"`

	// 异常GC问题 | GC issues
	GCIssues []string `json:"gc_issues" msgpack:"gc_issues"`

	// 内存使用问题 | Memory issues
	MemoryIssues []string `json:"memory_issues" msgpack:"memory_issues"`

	// 安全边界问题 | Security issues
	SecurityIssues []string `json:"security_issues" msgpack:"security_issues"`

	// 程序稳定性问题 | Stability issues
	StabilityIssues []string `json:"stability_issues" msgpack:"stability_issues"`

	// 总体评分 (0-100) | Overall score (0-100)
	OverallScore int `json:"overall_score" msgpack:"overall_score"`
}

// Analyzer GC管理器分析器 | GC manager analyzer
type Analyzer struct {
	manager *Manager
	report  *AnalysisReport
	mu      sync.RWMutex
}

// NewAnalyzer 创建新的分析器 | Create a new analyzer
func NewAnalyzer(manager *Manager) *Analyzer {
	return &Analyzer{
		manager: manager,
		report:  new(AnalysisReport),
	}
}

// Analyze 执行完整分析 | Execute full analysis
func (a *Analyzer) Analyze() *AnalysisReport {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.report = &AnalysisReport{
		AnalysisTime: time.Now(),
	}

	// 执行各项检查
	a.checkDataRaces()
	a.checkPerformance()
	a.checkGCBehavior()
	a.checkMemoryUsage()
	a.checkSecurity()
	a.checkStability()

	// 计算总体评分
	a.calculateOverallScore()

	return a.report
}

// checkDataRaces 检查数据竞争问题
func (a *Analyzer) checkDataRaces() {
	if a.manager == nil {
		a.report.DataRaceIssues = append(a.report.DataRaceIssues, "Manager is nil - potential null pointer access")
		return
	}

	// 检查原子操作的使用
	if atomic.LoadInt32(&a.manager.running) < 0 {
		a.report.DataRaceIssues = append(a.report.DataRaceIssues, "Invalid running state detected")
	}

	if atomic.LoadInt32(&a.manager.initialized) < 0 {
		a.report.DataRaceIssues = append(a.report.DataRaceIssues, "Invalid initialized state detected")
	}

	// 检查并发访问保护
	if a.manager.stats == nil {
		a.report.DataRaceIssues = append(a.report.DataRaceIssues, "Stats object is nil - potential concurrent access issue")
	}

	// 检查context和cancel的一致性
	if a.manager.ctx == nil && a.manager.cancel != nil {
		a.report.DataRaceIssues = append(a.report.DataRaceIssues, "Context is nil but cancel function exists")
	}

	if a.manager.ctx != nil && a.manager.cancel == nil {
		a.report.DataRaceIssues = append(a.report.DataRaceIssues, "Context exists but cancel function is nil")
	}
}

// checkPerformance 检查性能问题
func (a *Analyzer) checkPerformance() {
	if a.manager == nil || a.manager.config == nil {
		a.report.PerformanceIssues = append(a.report.PerformanceIssues, "Manager or config is nil")
		return
	}

	// 检查GC间隔设置
	if a.manager.config.Interval < time.Second {
		a.report.PerformanceIssues = append(a.report.PerformanceIssues,
			fmt.Sprintf("GC interval too short: %v (may cause performance degradation)", a.manager.config.Interval))
	}

	if a.manager.config.Interval > 30*time.Minute {
		a.report.PerformanceIssues = append(a.report.PerformanceIssues,
			fmt.Sprintf("GC interval too long: %v (may cause memory buildup)", a.manager.config.Interval))
	}

	// 检查重试配置
	if a.manager.config.MaxRetries > 10 {
		a.report.PerformanceIssues = append(a.report.PerformanceIssues,
			fmt.Sprintf("Too many retries configured: %d (may cause blocking)", a.manager.config.MaxRetries))
	}

	// 检查统计信息
	if a.manager.stats != nil {
		stats := a.manager.GetStats()
		if stats.AverageRunDuration > 5*time.Second {
			a.report.PerformanceIssues = append(a.report.PerformanceIssues,
				fmt.Sprintf("Average GC duration too long: %v", stats.AverageRunDuration))
		}

		if stats.TotalRuns > 0 {
			failureRate := float64(stats.FailedRuns) / float64(stats.TotalRuns)
			if failureRate > 0.1 {
				a.report.PerformanceIssues = append(a.report.PerformanceIssues,
					fmt.Sprintf("High failure rate: %.2f%%", failureRate*100))
			}
		}
	}
}

// checkGCBehavior 检查GC行为异常
func (a *Analyzer) checkGCBehavior() {
	if a.manager == nil || a.manager.strategy == nil {
		a.report.GCIssues = append(a.report.GCIssues, "Manager or strategy is nil")
		return
	}

	// 测试GC策略执行
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	start := time.Now()
	err := a.manager.strategy.Execute(ctx)
	duration := time.Since(start)

	if err != nil {
		a.report.GCIssues = append(a.report.GCIssues,
			fmt.Sprintf("GC strategy execution failed: %v", err))
	}

	if duration > 10*time.Second {
		a.report.GCIssues = append(a.report.GCIssues,
			fmt.Sprintf("GC execution took too long: %v", duration))
	}

	// 检查内存阈值设置
	if a.manager.config != nil && a.manager.config.MemoryThreshold > 0 {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)

		if a.manager.config.MemoryThreshold < m.Alloc {
			a.report.GCIssues = append(a.report.GCIssues,
				"Memory threshold is lower than current allocation")
		}
	}
}

// checkMemoryUsage 检查内存使用问题
func (a *Analyzer) checkMemoryUsage() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// 检查堆内存使用
	if m.HeapAlloc > 1024*1024*1024 { // 1GB
		a.report.MemoryIssues = append(a.report.MemoryIssues,
			fmt.Sprintf("High heap allocation: %d bytes", m.HeapAlloc))
	}

	// 检查GC频率
	if m.NumGC > 1000 {
		a.report.MemoryIssues = append(a.report.MemoryIssues,
			fmt.Sprintf("High GC count: %d", m.NumGC))
	}

	// 检查GC暂停时间
	// m.PauseNs 是一个循环缓冲区，最近的暂停时间在 (m.NumGC+255)%256 位置
	// 只有当 NumGC > 0 时才有有效的暂停时间数据
	if m.NumGC > 0 && m.PauseNs[(m.NumGC+255)%256] > 100*1000*1000 { // 100ms
		a.report.MemoryIssues = append(a.report.MemoryIssues,
			"Long GC pause time detected")
	}

	// 检查内存增长趋势
	if a.manager != nil && a.manager.stats != nil {
		stats := a.manager.GetStats()
		if stats.MemoryAfterGC > stats.MemoryBeforeGC {
			a.report.MemoryIssues = append(a.report.MemoryIssues,
				"Memory not being freed by GC")
		}
	}
}

// checkSecurity 检查安全边界问题
func (a *Analyzer) checkSecurity() {
	if a.manager == nil {
		a.report.SecurityIssues = append(a.report.SecurityIssues, "Manager is nil - potential security vulnerability")
		return
	}

	// 检查配置安全性
	if a.manager.config != nil {
		if a.manager.config.MaxRetries < 0 {
			a.report.SecurityIssues = append(a.report.SecurityIssues, "Negative retry count could cause infinite loops")
		}

		if a.manager.config.Interval <= 0 {
			a.report.SecurityIssues = append(a.report.SecurityIssues, "Zero or negative interval could cause resource exhaustion")
		}
	}

	// 检查goroutine泄漏风险
	if a.manager.IsRunning() && a.manager.ticker == nil {
		a.report.SecurityIssues = append(a.report.SecurityIssues, "Manager running but ticker is nil - potential goroutine leak")
	}

	// Go 1.26+ goroutine leak 检测 | Go 1.26+ goroutine leak detection
	a.checkGoroutineLeaks()

	// 检查context泄漏
	if a.manager.ctx != nil {
		select {
		case <-a.manager.ctx.Done():
			if a.manager.IsRunning() {
				a.report.SecurityIssues = append(a.report.SecurityIssues, "Context cancelled but manager still running")
			}
		default:
			// Context is still active
		}
	}
}

// checkGoroutineLeaks 检查 goroutine 泄漏 (Go 1.26+) | Check goroutine leaks (Go 1.26+)
func (a *Analyzer) checkGoroutineLeaks() {
	// 尝试获取 goroutineleak profile (Go 1.26+ 实验性功能)
	// Try to get goroutineleak profile (Go 1.26+ experimental feature)
	profile := pprof.Lookup("goroutineleak")
	if profile == nil {
		// goroutineleak profile 不可用，可能是 Go 版本不支持或未启用
		// goroutineleak profile not available, may not be supported or enabled
		return
	}

	// 检查是否有泄漏的 goroutine
	// Check if there are leaked goroutines
	if profile.Count() > 0 {
		a.report.SecurityIssues = append(a.report.SecurityIssues,
			fmt.Sprintf("Detected %d potentially leaked goroutines (Go 1.26+ leak detection)", profile.Count()))

		logger.Warn("Goroutine leak detected",
			zap.Int("leaked_count", profile.Count()),
			zap.String("detection_method", "Go 1.26+ goroutineleak profile"))
	}
}

// checkStability 检查程序稳定性问题
func (a *Analyzer) checkStability() {
	if a.manager == nil {
		a.report.StabilityIssues = append(a.report.StabilityIssues, "Manager is nil")
		return
	}

	// 检查初始化状态
	if atomic.LoadInt32(&a.manager.initialized) == 0 {
		a.report.StabilityIssues = append(a.report.StabilityIssues, "Manager not properly initialized")
	}

	// 检查运行状态一致性
	isRunning := a.manager.IsRunning()
	hasContext := a.manager.ctx != nil
	hasTicker := a.manager.ticker != nil

	if isRunning && (!hasContext || !hasTicker) {
		a.report.StabilityIssues = append(a.report.StabilityIssues, "Inconsistent running state")
	}

	if !isRunning && (hasContext || hasTicker) {
		a.report.StabilityIssues = append(a.report.StabilityIssues, "Resources not properly cleaned up")
	}

	// 检查done channel状态
	if a.manager.done != nil {
		select {
		case <-a.manager.done:
			if isRunning {
				a.report.StabilityIssues = append(a.report.StabilityIssues, "Done channel closed but manager still running")
			}
		default:
			// Channel is still open
		}
	}
}

// calculateOverallScore 计算总体评分
func (a *Analyzer) calculateOverallScore() {
	totalIssues := len(a.report.DataRaceIssues) +
		len(a.report.PerformanceIssues) +
		len(a.report.GCIssues) +
		len(a.report.MemoryIssues) +
		len(a.report.SecurityIssues) +
		len(a.report.StabilityIssues)

	// 基础分数100，每个问题扣分
	score := 100 - (totalIssues * 5)
	if score < 0 {
		score = 0
	}

	a.report.OverallScore = score
}

// LogReport 记录分析报告到日志 | Log analysis report
func (a *Analyzer) LogReport(report *AnalysisReport) {
	// 只记录有问题的情况
	totalIssues := len(report.DataRaceIssues) + len(report.PerformanceIssues) +
		len(report.GCIssues) + len(report.MemoryIssues) +
		len(report.SecurityIssues) + len(report.StabilityIssues)

	if totalIssues > 0 {
		logger.Warn("GC Manager Analysis Report",
			zap.Int("overall_score", report.OverallScore),
			zap.Int("total_issues", totalIssues))

		// 只记录严重问题
		for _, issue := range report.DataRaceIssues {
			logger.Warn("Data Race Issue", zap.String("issue", issue))
		}
		for _, issue := range report.SecurityIssues {
			logger.Warn("Security Issue", zap.String("issue", issue))
		}
	}
	for _, issue := range report.GCIssues {
		logger.Warn("GC Issue", zap.String("issue", issue))
	}
	for _, issue := range report.MemoryIssues {
		logger.Warn("Memory Issue", zap.String("issue", issue))
	}
	for _, issue := range report.SecurityIssues {
		logger.Error("Security Issue", zap.String("issue", issue))
	}
	for _, issue := range report.StabilityIssues {
		logger.Error("Stability Issue", zap.String("issue", issue))
	}
}
