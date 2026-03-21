/**
 * charts.js — Chart.js 图表初始化工具 | Chart.js initialization utilities
 * 主题感知的图表默认配置、响应式配置、通用图表创建函数
 * Theme-aware chart defaults, responsive config, common chart creation functions
 *
 * 依赖 Dependencies: Chart.js 4.15.1+ (vendor/chart.min.js)
 */

'use strict'

/* ========================================
   主题感知颜色 | Theme-aware colors
   ======================================== */

/**
 * getChartColors 获取当前主题下的图表颜色 | Get chart colors for current theme
 * @returns {Object} 颜色配置 Color configuration
 */
const getChartColors = () => {
    const style = getComputedStyle(document.documentElement)
    const isDark = document.documentElement.getAttribute('data-bs-theme') === 'dark'
    return {
        primary: style.getPropertyValue('--color-primary').trim() || '#154979',
        primaryLight: style.getPropertyValue('--color-primary-light').trim() || '#2A5A8C',
        success: style.getPropertyValue('--color-success').trim() || '#2E7D32',
        warning: style.getPropertyValue('--color-warning').trim() || '#E65100',
        danger: style.getPropertyValue('--color-danger').trim() || '#C62828',
        info: style.getPropertyValue('--color-info').trim() || '#1565C0',
        text: style.getPropertyValue('--color-text-primary').trim() || (isDark ? '#E0E0E0' : '#1A1A1A'),
        textSecondary: style.getPropertyValue('--color-text-secondary').trim() || (isDark ? '#A0A0A0' : '#666666'),
        border: style.getPropertyValue('--color-border').trim() || (isDark ? '#3A3A3A' : '#D9D9D9'),
        bg: style.getPropertyValue('--color-bg-card').trim() || (isDark ? '#1E1E1E' : '#FFFFFF')
    }
}

/** CHART_PALETTE 图表调色板 | Chart color palette */
const CHART_PALETTE = [
    '#154979', '#2E7D32', '#E65100', '#1565C0',
    '#C62828', '#6A1B9A', '#00838F', '#EF6C00',
    '#283593', '#2E7D32', '#AD1457', '#4E342E'
]

/* ========================================
   全局默认配置 | Global default configuration
   ======================================== */

/** setChartDefaults 设置 Chart.js 全局默认配置 | Set Chart.js global defaults */
const setChartDefaults = () => {
    if (typeof Chart === 'undefined') return
    const colors = getChartColors()
    Chart.defaults.font.family = getComputedStyle(document.documentElement)
        .getPropertyValue('--font-family').trim() || 'system-ui, sans-serif'
    Chart.defaults.font.size = 12
    Chart.defaults.color = colors.textSecondary
    Chart.defaults.borderColor = colors.border
    Chart.defaults.responsive = true
    Chart.defaults.maintainAspectRatio = false
    Chart.defaults.animation.duration = 300
    Chart.defaults.plugins.legend.labels.usePointStyle = true
    Chart.defaults.plugins.legend.labels.padding = 16
    Chart.defaults.plugins.tooltip.backgroundColor = colors.text
    Chart.defaults.plugins.tooltip.titleFont = { weight: '600' }
    Chart.defaults.plugins.tooltip.cornerRadius = 4
    Chart.defaults.plugins.tooltip.padding = 8
}

/* ========================================
   通用图表创建函数 | Common chart creation functions
   ======================================== */

/** createLineChart 创建折线图 | Create line chart */
const createLineChart = (canvas, config) => {
    if (typeof Chart === 'undefined' || !canvas) return null
    const colors = getChartColors()
    const datasets = config.datasets.map((ds, i) => ({
        label: ds.label, data: ds.data,
        borderColor: ds.color || CHART_PALETTE[i % CHART_PALETTE.length],
        backgroundColor: (ds.color || CHART_PALETTE[i % CHART_PALETTE.length]) + '20',
        borderWidth: 2, pointRadius: 3, pointHoverRadius: 5, tension: 0.3, fill: ds.fill || false
    }))
    return new Chart(canvas, {
        type: 'line', data: { labels: config.labels, datasets },
        options: {
            scales: {
                x: { grid: { color: colors.border } },
                y: { grid: { color: colors.border }, beginAtZero: config.beginAtZero !== false }
            },
            ...config.options
        }
    })
}

/** createBarChart 创建柱状图 | Create bar chart */
const createBarChart = (canvas, config) => {
    if (typeof Chart === 'undefined' || !canvas) return null
    const colors = getChartColors()
    const datasets = config.datasets.map((ds, i) => ({
        label: ds.label, data: ds.data,
        backgroundColor: ds.color || CHART_PALETTE[i % CHART_PALETTE.length] + 'CC',
        borderColor: ds.color || CHART_PALETTE[i % CHART_PALETTE.length],
        borderWidth: 1, borderRadius: 2
    }))
    return new Chart(canvas, {
        type: 'bar', data: { labels: config.labels, datasets },
        options: {
            scales: { x: { grid: { display: false } }, y: { grid: { color: colors.border }, beginAtZero: true } },
            ...config.options
        }
    })
}

/** createDoughnutChart 创建环形图 | Create doughnut chart */
const createDoughnutChart = (canvas, config) => {
    if (typeof Chart === 'undefined' || !canvas) return null
    return new Chart(canvas, {
        type: 'doughnut',
        data: {
            labels: config.labels,
            datasets: [{
                data: config.data,
                backgroundColor: config.colors || CHART_PALETTE.slice(0, config.data.length),
                borderWidth: 0
            }]
        },
        options: {
            cutout: config.cutout || '65%',
            plugins: { legend: { position: config.legendPosition || 'bottom' } }, ...config.options
        }
    })
}

/** updateChartTheme 更新所有图表的主题颜色 | Update theme colors for all charts */
const updateChartTheme = () => {
    if (typeof Chart === 'undefined') return
    setChartDefaults()
    Object.values(Chart.instances).forEach((chart) => {
        chart.update()
    })
}

// 监听主题变化 | Listen for theme changes
const themeObserver = new MutationObserver((mutations) => {
    mutations.forEach((m) => {
        if (m.attributeName === 'data-bs-theme') updateChartTheme()
    })
})

// DOMContentLoaded 初始化 | DOMContentLoaded initialization
document.addEventListener('DOMContentLoaded', () => {
    setChartDefaults()
    themeObserver.observe(document.documentElement, { attributes: true, attributeFilter: ['data-bs-theme'] })
})

