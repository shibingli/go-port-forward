/**
 * app.js — 应用主脚本 | Main application script
 * Alpine.js 组件注册、全局事件监听、通用工具函数
 * Alpine components, global event listeners, utility functions
 *
 * 加载顺序 Load order:
 * 1. lib/bootstrap.bundle.min.js
 * 2. lib/htmx.min.js
 * 3. lib/alpine-collapse.min.js (defer, 必须在 Alpine 核心之前 must load before Alpine core)
 * 4. lib/alpine.min.js (defer)
 * 5. app.js (本文件 this file)
 * 6. htmx-ext.js
 * 7. charts.js (按需 on demand)
 * 8. user-preferences.js (按需 on demand)
 */

'use strict'

/* ========================================
   常量 | Constants
   ======================================== */
const TOAST_DURATION = 4000
const TOAST_CONTAINER_ID = 'toast-container'

/* ========================================
   全局顶部进度条 | Global top progress bar (NProgress-like)
   统一管理 HTMX 请求和 fetch API 请求的进度反馈
   Unified progress feedback for HTMX requests and fetch API requests
   ======================================== */

/**
 * TopProgress 全局顶部进度条管理器 | Global top progress bar manager
 * 类似 NProgress 的效果：启动时快速推进到 ~80%，完成时推进到 100% 并淡出
 * NProgress-like effect: quickly advances to ~80% on start, completes to 100% and fades out
 *
 * 支持并发请求计数，多个请求同时进行时进度条保持显示
 * Supports concurrent request counting, progress bar stays visible during multiple requests
 */
const TopProgress = (() => {
        let _count = 0       // 并发请求计数 | Concurrent request counter
        let _value = 0       // 当前进度值 0~1 | Current progress value 0~1
        let _timer = null    // 涓流定时器 | Trickle timer
        let _hiding = false  // 是否正在隐藏 | Whether hiding in progress
        let _safetyTimer = null  // 安全超时定时器 | Safety timeout timer

        // 安全超时时间（ms）：防止计数器泄漏导致进度条永远卡住
        // Safety timeout (ms): prevents counter leak from keeping progress bar stuck forever
        const SAFETY_TIMEOUT = 15000

        const _bar = () => document.querySelector('.htmx-progress')

        /**
         * _set 设置进度条位置 | Set progress bar position
         * @param {number} n - 进度值 0~1 Progress value 0~1
         */
        const _set = (n) => {
            _value = Math.max(0, Math.min(n, 1))
            const el = _bar()
            if (!el) return
            el.style.transition = _value === 0 ? 'none' : 'width 0.25s ease, opacity 0.15s ease'
            el.style.width = (_value * 100) + '%'
            el.style.opacity = _value > 0 ? '1' : '0'
            // 强制重绘 | Force reflow
            void el.offsetWidth
        }

        /**
         * _inc 自动递增进度（涓流效果）| Auto-increment progress (trickle effect)
         * 越接近 100% 增量越小，永远不会自动到达 100%
         * Increment gets smaller as it approaches 100%, never auto-reaches 100%
         */
        const _inc = () => {
            let amount
            if (_value < 0.2) amount = 0.1
            else if (_value < 0.5) amount = 0.04
            else if (_value < 0.8) amount = 0.02
            else if (_value < 0.99) amount = 0.005
            else return // 不再增长 | Stop growing
            _set(_value + amount)
        }

        /**
         * _startTrickle 启动涓流定时器 | Start trickle timer
         * 每 300ms 自动递增一次进度
         * Auto-increments progress every 300ms
         */
        const _startTrickle = () => {
            if (_timer) return
            _timer = setInterval(_inc, 300)
        }

        /** _stopTrickle 停止涓流定时器 | Stop trickle timer */
        const _stopTrickle = () => {
            if (_timer) {
                clearInterval(_timer)
                _timer = null
            }
        }

        /** _clearSafetyTimer 清除安全超时 | Clear safety timeout */
        const _clearSafetyTimer = () => {
            if (_safetyTimer) {
                clearTimeout(_safetyTimer)
                _safetyTimer = null
            }
        }

        /**
         * _startSafetyTimer 启动安全超时 | Start safety timeout
         * 如果进度条在 SAFETY_TIMEOUT 时间内未完成，强制重置
         * If progress bar doesn't complete within SAFETY_TIMEOUT, force reset
         */
        const _startSafetyTimer = () => {
            _clearSafetyTimer()
            _safetyTimer = setTimeout(() => {
                if (_count > 0) {
                    _count = 0
                    _stopTrickle()
                    _hiding = true
                    _set(1)
                    setTimeout(() => {
                        if (!_hiding) return
                        const el = _bar()
                        if (el) {
                            el.style.transition = 'opacity 0.25s ease'
                            el.style.opacity = '0'
                        }
                        setTimeout(() => {
                            if (!_hiding) return
                            _set(0)
                            _hiding = false
                        }, 250)
                    }, 200)
                }
            }, SAFETY_TIMEOUT)
        }

        return {
            /**
             * start 开始进度条 | Start progress bar
             * 并发安全：多次调用只增加计数，进度条保持显示
             * Concurrency-safe: multiple calls only increment counter, bar stays visible
             */
            start() {
                _count++
                if (_count === 1) {
                    _hiding = false
                    _set(0)
                    // 下一帧启动动画，确保 transition 生效 | Start animation on next frame
                    requestAnimationFrame(() => {
                        _set(0.08)
                        _startTrickle()
                    })
                    // 启动安全超时 | Start safety timeout
                    _startSafetyTimer()
                }
            },

            /**
             * done 完成进度条 | Complete progress bar
             * 并发安全：仅当所有请求完成后才执行完成动画
             * Concurrency-safe: only runs completion animation when all requests finish
             */
            done() {
                if (_count <= 0) return
                _count--
                if (_count > 0) return
                _clearSafetyTimer()
                _stopTrickle()
                _hiding = true
                _set(1)
                setTimeout(() => {
                    if (!_hiding) return
                    const el = _bar()
                    if (el) {
                        el.style.transition = 'opacity 0.25s ease'
                        el.style.opacity = '0'
                    }
                    setTimeout(() => {
                        if (!_hiding) return
                        _set(0)
                        _hiding = false
                    }, 250)
                }, 200)
            },

            /**
             * isActive 是否有活跃请求 | Whether there are active requests
             * @returns {boolean}
             */
            isActive() {
                return _count > 0
            }
        }
    })()

    /* ========================================
       fetch 自动进度条 | Auto progress bar for fetch API
       拦截所有 fetch 请求，自动触发 TopProgress
       Intercepts all fetch requests, auto-triggers TopProgress
       ======================================== */

;(() => {
    const _origFetch = window.fetch
    window.fetch = function(...args) {
        TopProgress.start()
        return _origFetch.apply(this, args).then(
            (r) => {
                TopProgress.done()
                return r
            },
            (e) => {
                TopProgress.done()
                throw e
            }
        )
    }
})()

/* ========================================
   页面导航进度条 | Page navigation progress bar
   拦截内部链接点击，在全页面跳转时显示进度条
   Intercepts internal link clicks, shows progress bar during full-page navigation
   ======================================== */

document.addEventListener('click', (e) => {
    // 查找最近的 <a> 元素 | Find closest <a> element
    const link = e.target.closest('a')
    if (!link) return

    const href = link.getAttribute('href')
    if (!href) return

    // 排除：锚点、javascript:、外部链接、新窗口、下载 | Exclude: anchors, javascript:, external, new tab, download
    if (href.startsWith('#') || href.startsWith('javascript:')) return
    if (link.target === '_blank' || link.hasAttribute('download')) return

    // 排除：HTMX 请求（已由 htmx:beforeRequest 处理）| Exclude: HTMX requests (handled by htmx:beforeRequest)
    if (link.hasAttribute('hx-get') || link.hasAttribute('hx-post') ||
        link.hasAttribute('hx-put') || link.hasAttribute('hx-patch') ||
        link.hasAttribute('hx-delete')) return

    // 排除：hx-boost 链接（自身或祖先元素）| Exclude: hx-boost links (self or ancestor)
    if (link.closest('[hx-boost="true"]')) return

    // 排除：修饰键（Ctrl/Meta/Shift 点击在新标签页打开）| Exclude: modifier keys (open in new tab)
    if (e.ctrlKey || e.metaKey || e.shiftKey || e.altKey) return

    // 排除：外部链接 | Exclude: external links
    try {
        const url = new URL(href, window.location.origin)
        if (url.origin !== window.location.origin) return
    } catch {
        return
    }

    TopProgress.start()
})

/* ========================================
   Toast 通知系统 | Toast notification system
   ======================================== */

/**
 * showToast 显示 Toast 通知 | Show toast notification
 * @param {string} level - 级别 Level: success | danger | warning | info
 * @param {string} message - 消息内容 Message content
 */
const showToast = (level, message) => {
    let container = document.getElementById(TOAST_CONTAINER_ID)
    if (!container) {
        container = document.createElement('div')
        container.id = TOAST_CONTAINER_ID
        container.className = 'toast-container'
        document.body.appendChild(container)
    }

    const iconMap = {
        success: 'bi-check-circle-fill',
        danger: 'bi-exclamation-triangle-fill',
        warning: 'bi-exclamation-circle-fill',
        info: 'bi-info-circle-fill'
    }

    const toast = document.createElement('div')
    toast.className = `toast toast--${level} show`
    toast.setAttribute('role', 'alert')
    toast.innerHTML = `
    <div class="toast-body d-flex align-items-center">
      <i class="bi ${iconMap[level] || iconMap.info} toast__icon"></i>
      <span>${escapeHtml(message)}</span>
      <button type="button" class="btn-close btn-close-sm ms-auto"
              onclick="this.closest('.toast').remove()"></button>
    </div>
  `

    container.appendChild(toast)

    // 自动移除 | Auto remove
    setTimeout(() => {
        toast.classList.add('fade')
        setTimeout(() => toast.remove(), 300)
    }, TOAST_DURATION)
}

// 监听后端 HTMX 触发的 Toast 事件 | Listen for backend HTMX-triggered Toast events
document.addEventListener('showToast', (e) => {
    const { level, message } = e.detail
    showToast(level, message)
})

/* ========================================
   确认对话框 | Confirm dialog
   ======================================== */

/**
 * showConfirm 显示确认对话框 | Show confirmation dialog
 * @param {string} message - 确认消息 Confirmation message
 * @param {Function} onOk - 确认回调 Confirm callback
 * @param {string} [level='danger'] - 级别 Level: danger | warning
 */
const showConfirm = (message, onOk, level = 'danger') => {
    const modalEl = document.getElementById('confirm-modal')
    if (!modalEl) return

    const msgEl = modalEl.querySelector('.confirm-dialog__message')
    const iconEl = modalEl.querySelector('.confirm-dialog__icon')
    const okBtn = modalEl.querySelector('[data-confirm-ok]')
    const cancelBtn = modalEl.querySelector('#confirm-cancel-btn')

    if (msgEl) msgEl.textContent = message
    // 更新按钮文字为当前语言 | Update button text to current language
    if (cancelBtn) cancelBtn.textContent = typeof _t === 'function' ? _t('取消', 'Cancel') : '取消'
    if (okBtn) okBtn.textContent = typeof _t === 'function' ? _t('确认删除', 'Confirm Delete') : '确认删除'
    if (iconEl) {
        iconEl.className = `confirm-dialog__icon confirm-dialog__icon--${level} mb-3`
    }

    // 移除旧的事件监听 | Remove old event listener
    const newOkBtn = okBtn.cloneNode(true)
    okBtn.parentNode.replaceChild(newOkBtn, okBtn)
    newOkBtn.addEventListener('click', () => {
        const modal = bootstrap.Modal.getInstance(modalEl)
        if (modal) {
            // 移除焦点，避免 aria-hidden 警告 | Blur focus to avoid aria-hidden warning
            if (document.activeElement && modalEl.contains(document.activeElement)) {
                document.activeElement.blur()
            }
            modal.hide()
        }
        if (typeof onOk === 'function') onOk()
    })

    const modal = bootstrap.Modal.getOrCreateInstance(modalEl)
    modal.show()
}

/* ========================================
   通用工具函数 | Utility functions
   ======================================== */

/**
 * escapeHtml HTML 转义 | HTML escape
 * 使用正则替换方案，避免每次调用创建 DOM 元素，提升性能
 * Uses regex replacement to avoid creating DOM elements on each call for better performance
 * @param {string} str - 原始字符串 Raw string
 * @returns {string} 转义后的字符串 Escaped string
 */
const escapeHtml = (str) => {
    if (!str) return ''
    const escapeMap = { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', '\'': '&#39;' }
    return String(str).replace(/[&<>"']/g, (ch) => escapeMap[ch])
}

/* ========================================
   侧边栏管理 | Sidebar management
   ======================================== */

/**
 * toggleSidebar 切换侧边栏展开/收起 | Toggle sidebar expand/collapse
 * 同时将折叠状态持久化到 SecureStore | Also persists collapsed state to SecureStore
 */
const toggleSidebar = () => {
    const sidebar = document.querySelector('.layout__sidebar')
    const layout = document.querySelector('.layout')
    if (!sidebar) return

    const isMobile = window.innerWidth < 992

    if (isMobile) {
        sidebar.classList.toggle('layout__sidebar--mobile-open')
        const overlay = document.querySelector('.sidebar-overlay')
        if (overlay) overlay.classList.toggle('sidebar-overlay--visible')
    } else {
        const willCollapse = !sidebar.classList.contains('layout__sidebar--collapsed')
        sidebar.classList.toggle('layout__sidebar--collapsed')
        if (layout) layout.classList.toggle('layout--collapsed')

        // 持久化折叠状态到 SecureStore | Persist collapsed state to SecureStore
        SecureStore.set('sidebar_collapsed', willCollapse ? 'true' : 'false')

        // 折叠时清除展开的菜单组，避免 flyout 立即弹出 | Clear open group when collapsing to prevent immediate flyout
        if (willCollapse && typeof Alpine !== 'undefined') {
            const alpineData = Alpine.$data(sidebar)
            if (alpineData) alpineData.openGroup = ''
        }
    }
}

/**
 * closeMobileSidebar 关闭移动端侧边栏 | Close mobile sidebar
 */
const closeMobileSidebar = () => {
    const sidebar = document.querySelector('.layout__sidebar')
    const overlay = document.querySelector('.sidebar-overlay')
    if (sidebar) sidebar.classList.remove('layout__sidebar--mobile-open')
    if (overlay) overlay.classList.remove('sidebar-overlay--visible')
}

// 窗口大小变化时关闭移动端侧边栏 | Close mobile sidebar on window resize
window.addEventListener('resize', () => {
    if (window.innerWidth >= 992) {
        closeMobileSidebar()
    }
})

/* ========================================
   颜色工具函数 | Color utility functions
   ======================================== */

/**
 * hexToRgb 将 HEX 颜色转换为 RGB 字符串 | Convert HEX color to RGB string
 * @param {string} hex - HEX 颜色值 HEX color value (e.g., '#154979')
 * @returns {string} RGB 字符串 RGB string (e.g., '21, 73, 121')
 */
const hexToRgb = (hex) => {
    const result = /^#?([a-f\d]{2})([a-f\d]{2})([a-f\d]{2})$/i.exec(hex)
    if (!result) return '21, 73, 121'
    return `${parseInt(result[1], 16)}, ${parseInt(result[2], 16)}, ${parseInt(result[3], 16)}`
}

/**
 * adjustColor 调整颜色亮度 | Adjust color brightness
 * @param {string} hex - HEX 颜色值 HEX color value
 * @param {number} amount - 调整量 Adjustment amount (-255 to 255)
 * @returns {string} 调整后的 HEX 颜色 Adjusted HEX color
 */
const adjustColor = (hex, amount) => {
    const num = parseInt(hex.replace('#', ''), 16)
    const r = Math.min(255, Math.max(0, ((num >> 16) & 0xFF) + amount))
    const g = Math.min(255, Math.max(0, ((num >> 8) & 0xFF) + amount))
    const b = Math.min(255, Math.max(0, (num & 0xFF) + amount))
    return `#${((1 << 24) + (r << 16) + (g << 8) + b).toString(16).slice(1)}`
}

/**
 * applyPrimaryColor 应用自定义主色调到页面 | Apply custom primary color to page
 * 计算并注入完整的颜色变量集（主色、亮色、暗色、RGB、侧边栏背景等）
 * Computes and injects full color variable set (primary, light, dark, RGB, sidebar bg, etc.)
 * @param {string} hex - HEX 颜色值 HEX color value (e.g., '#e59524')
 */
const applyPrimaryColor = (hex) => {
    const styleId = 'user-theme-overrides'
    let styleEl = document.getElementById(styleId)

    if (!hex) {
        // 移除自定义颜色，恢复默认 | Remove custom color, restore default
        if (styleEl) styleEl.remove()
        return
    }

    if (!styleEl) {
        styleEl = document.createElement('style')
        styleEl.id = styleId
        document.head.appendChild(styleEl)
    }

    const rgb = hexToRgb(hex)
    const light = adjustColor(hex, 40)
    const dark = adjustColor(hex, -30)

    styleEl.textContent = `:root, [data-bs-theme="light"], [data-bs-theme="dark"] {
  --color-primary: ${hex};
  --color-primary-light: ${light};
  --color-primary-dark: ${dark};
  --color-primary-bg: ${hex}10;
  --color-primary-rgb: ${rgb};
  --color-bg-sidebar: ${dark};
  --bs-primary: ${hex};
  --bs-primary-rgb: ${rgb};
}`
}

/**
 * initUserPrimaryColor 初始化用户自定义主色调 | Initialize user custom primary color
 * 优先从 SecureStore 读取，其次从 HTML data 属性读取（服务端注入）
 * Reads from SecureStore first, then from HTML data attribute (server-side injection)
 */
const initUserPrimaryColor = () => {
    let color = ''

    // 优先从 SecureStore 读取 | Read from SecureStore first
    try {
        const saved = SecureStore.getSync('user_preferences')
        if (saved && saved.primary_color) {
            color = saved.primary_color
        }
    } catch {
        // 忽略解析错误 | Ignore parse errors
    }

    // 其次从 HTML data 属性读取（服务端注入）| Fallback to HTML data attribute (server-side)
    if (!color) {
        color = document.documentElement.getAttribute('data-user-primary-color') || ''
    }

    if (color) applyPrimaryColor(color)
}

/* ========================================
   主题初始化 | Theme initialization
   ======================================== */

/**
 * initTheme 初始化主题模式 | Initialize theme mode
 * 注：auto 模式的主题解析已在 base.html <head> 内联脚本中同步完成（防闪烁）
 * 此函数仅负责注册系统主题变化监听器
 * Note: auto mode theme resolution is done synchronously in base.html <head> inline script (FOUC prevention)
 * This function only registers the system theme change listener
 */
const initTheme = () => {
    const html = document.documentElement

    // 如果是 auto 模式，注册系统主题变化监听器
    // If auto mode, register system theme change listener
    if (html.getAttribute('data-bs-theme-mode') === 'auto') {
        window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', (e) => {
            if (html.getAttribute('data-bs-theme-mode') === 'auto') {
                html.setAttribute('data-bs-theme', e.matches ? 'dark' : 'light')
            }
        })
    }
}

/* ========================================
   Alpine.js 全局数据/组件 | Alpine.js global data/components
   ======================================== */

// 在 DOMContentLoaded 时立即处理侧边栏高亮，避免闪烁
// Process sidebar highlighting immediately on DOMContentLoaded to avoid flashing
document.addEventListener('DOMContentLoaded', () => {
    const currentPath = window.location.pathname
    let activeGroupName = ''

    // 收集所有匹配的链接，选择最长（最精确）的匹配
    // Collect all matching links, pick the longest (most specific) match
    let bestMatch = null
    let bestMatchLen = 0
    document.querySelectorAll('.sidebar__link').forEach((link) => {
        const href = link.getAttribute('href')
        if (!href || href === '/') return
        // 精确匹配或路径前缀匹配（href 后紧跟 / 或路径结束）
        // Exact match or path-prefix match (href followed by / or end of path)
        const isExact = currentPath === href
        const isPrefix = currentPath.startsWith(href + '/')
        if ((isExact || isPrefix) && href.length > bestMatchLen) {
            bestMatch = link
            bestMatchLen = href.length
        }
    })

    // 仅高亮最精确匹配的菜单项 | Only highlight the most specific match
    if (bestMatch) {
        bestMatch.classList.add('sidebar__link--active')
        // 查找所属分组名称 | Find parent group name
        const submenu = bestMatch.closest('.sidebar__submenu')
        if (submenu) {
            const group = submenu.closest('.sidebar__group')
            if (group) {
                const title = group.querySelector('.sidebar__group-title')
                if (title) {
                    const clickAttr = title.getAttribute('@click') || ''
                    const match = clickAttr.match(/openGroup === '(\w+)'/)
                    if (match) activeGroupName = match[1]
                }
            }
        }
    } else if (currentPath === '/') {
        // 首页精确匹配 | Homepage exact match
        const homeLink = document.querySelector('.sidebar__link[href="/"]')
        if (homeLink) homeLink.classList.add('sidebar__link--active')
    }

    // 自动展开包含当前页面的分组 | Auto-expand group containing current page
    if (activeGroupName) {
        SecureStore.set('sidebar_open_group', activeGroupName)
    }
})

// 等待 Alpine.js 加载后注册组件 | Register components after Alpine.js loads
document.addEventListener('alpine:init', () => {
    // 侧边栏状态（从 SecureStore 同步）| Sidebar state (synced from SecureStore)
    const sidebarEl = document.querySelector('.layout__sidebar')
    Alpine.store('sidebar', {
        collapsed: sidebarEl ? sidebarEl.classList.contains('layout__sidebar--collapsed') : false,
        toggle() {
            this.collapsed = !this.collapsed
            toggleSidebar()
        }
    })

    // 下拉菜单组件 | Dropdown component
    Alpine.data('dropdown', () => ({
        open: false,
        toggle() {
            this.open = !this.open
        },
        close() {
            this.open = false
        }
    }))

    // 标记 Alpine.js 已初始化，启用过渡动画 | Mark Alpine.js as initialized, enable transitions
    setTimeout(() => {
        const sidebar = document.querySelector('.layout__sidebar')
        if (sidebar) {
            sidebar.classList.add('alpine-initialized')
        }
    }, 100)
})

/* ========================================
   DOMContentLoaded 初始化 | DOMContentLoaded initialization
   ======================================== */
document.addEventListener('DOMContentLoaded', () => {
    // 注意：侧边栏折叠状态已由 app.html 内联脚本同步恢复，无需在此重复调用
    // Note: Sidebar collapsed state is synchronously restored by inline script in app.html, no need to call here
    initTheme()
    // 初始化用户自定义主色调 | Initialize user custom primary color
    initUserPrimaryColor()

    // 异步初始化 SecureStore（打开 IndexedDB，生成 AES-GCM 密钥）
    // Async initialize SecureStore (open IndexedDB, generate AES-GCM encryption key)
    SecureStore.init().catch(() => {
        // 初始化失败时降级为 localStorage 模式，不影响页面功能
        // Falls back to localStorage mode on failure, does not affect page functionality
    })
})

