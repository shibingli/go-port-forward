/**
 * htmx-ext.js — HTMX 扩展和事件处理 | HTMX extensions and event handling
 * CSRF Token 注入、错误处理、401/403 处理、加载指示器、422 内容交换、分页滚动
 * CSRF token injection, error handling, 401/403 processing, loading indicators,
 * 422 content swap, pagination scroll management
 */

'use strict'

/* ========================================
   国际化辅助 | i18n helper
   ======================================== */
const _isEn = () => document.documentElement.lang === 'en'

/* ========================================
   CSRF Token 注入 | CSRF Token injection
   ======================================== */

/**
 * getCSRFToken 获取 CSRF Token | Get CSRF token
 * 从 meta 标签或 cookie 中读取
 * Read from meta tag or cookie
 * @returns {string} CSRF token
 */
const getCSRFToken = () => {
    // 优先从 meta 标签获取 | Prefer meta tag
    const meta = document.querySelector('meta[name="csrf-token"]')
    if (meta) return meta.getAttribute('content')

    // 从 cookie 获取 | Fallback to cookie
    const match = document.cookie.match(/(?:^|;\s*)csrf_token=([^;]*)/)
    return match ? decodeURIComponent(match[1]) : ''
}

// 在所有 HTMX 请求中注入 CSRF Token 和 Authorization | Inject CSRF token and Authorization in all HTMX requests
document.addEventListener('htmx:configRequest', (e) => {
    // 注入 CSRF Token | Inject CSRF token
    const csrfToken = getCSRFToken()
    if (csrfToken) {
        e.detail.headers['X-CSRF-Token'] = csrfToken
    }

    // 注入 Authorization Bearer Token（从 SecureStore 读取）| Inject Authorization Bearer token (from SecureStore)
    const accessToken = SecureStore.getSync('access_token')
    if (accessToken) {
        e.detail.headers['Authorization'] = 'Bearer ' + accessToken
    }
})

/* ========================================
   错误处理 | Error handling
   ======================================== */

// HTMX 请求错误处理 | HTMX request error handling
document.addEventListener('htmx:responseError', (e) => {
    const { xhr } = e.detail
    const status = xhr.status

    if (status === 401) {
        // 会话过期，重定向到登录页 | Session expired, redirect to login
        const loginUrl = '/login?redirect=' + encodeURIComponent(window.location.pathname)
        window.location.href = loginUrl
        return
    }

    if (status === 403) {
        // 权限不足 | Access denied
        showToast('danger', _isEn() ? 'Access Denied' : '权限不足')
        return
    }

    // 422 由 htmx:beforeSwap 处理内容交换 | 422 handled by htmx:beforeSwap for content swap
    if (status === 422) return

    if (status === 429) {
        // 请求过于频繁 | Too many requests
        showToast('warning', _isEn() ? 'Too many requests, please try later' : '请求过于频繁，请稍后重试')
        return
    }

    if (status >= 500) {
        // 服务器错误 | Server error
        showToast('danger', _isEn() ? 'Server error, please try later' : '服务器错误，请稍后重试')
        return
    }

    // 其他错误 | Other errors
    showToast('danger', _isEn() ? 'Request failed' : '请求失败')
})

// HTMX 网络错误处理 | HTMX network error handling
// 注意：网络错误时 htmx:afterRequest 可能不触发，必须在此调用 done() 保持计数平衡
// Note: htmx:afterRequest may not fire on network error, must call done() here to keep counter balanced
document.addEventListener('htmx:sendError', () => {
    TopProgress.done()
    showToast('danger', _isEn() ? 'Network error, please check your connection' : '网络错误，请检查网络连接')
})

// HTMX 请求超时处理 | HTMX request timeout handling
document.addEventListener('htmx:timeout', () => {
    showToast('warning', _isEn() ? 'Request timeout, please try again' : '请求超时，请重试')
})

/* ========================================
   加载指示器 | Loading indicators
   使用 TopProgress 全局进度条（定义在 app.js）
   Uses TopProgress global progress bar (defined in app.js)
   ======================================== */

// HTMX 请求开始 → 启动进度条 | HTMX request start → start progress bar
document.addEventListener('htmx:beforeRequest', () => {
    TopProgress.start()
})

// HTMX 请求结束 → 完成进度条 | HTMX request end → complete progress bar
document.addEventListener('htmx:afterRequest', () => {
    TopProgress.done()
})

/* ========================================
   HTMX 后处理 | HTMX post-processing
   ======================================== */

// HTMX 内容加载后重新初始化 Bootstrap 组件 | Reinitialize Bootstrap components after HTMX content load
document.addEventListener('htmx:afterSettle', (e) => {
    const target = e.detail.target
    if (!target) return

    // 初始化 tooltips | Initialize tooltips
    const tooltips = target.querySelectorAll('[data-bs-toggle="tooltip"]')
    tooltips.forEach((el) => new bootstrap.Tooltip(el))

    // 初始化 popovers | Initialize popovers
    const popovers = target.querySelectorAll('[data-bs-toggle="popover"]')
    popovers.forEach((el) => new bootstrap.Popover(el))
})

/* ========================================
   htmx:beforeSwap — 状态码处理 | Status code handling
   支持 204、422 内容交换 | Support 204, 422 content swap
   ======================================== */
document.addEventListener('htmx:beforeSwap', (e) => {
    const { xhr } = e.detail

    if (xhr.status === 204) {
        // 204 No Content — 不替换内容 | Don't swap content
        e.detail.shouldSwap = false
        return
    }

    if (xhr.status === 422) {
        // 422 Unprocessable Entity — 允许服务端返回验证错误 HTML 片段进行交换
        // Allow server to return validation error HTML fragments for swap
        const contentType = xhr.getResponseHeader('Content-Type') || ''
        if (contentType.includes('text/html')) {
            // 服务端返回 HTML 内容，允许交换（显示表单验证错误）| Server returned HTML, allow swap (show form validation errors)
            e.detail.shouldSwap = true
            e.detail.isError = false
        } else {
            // 非 HTML 响应，尝试解析 JSON 显示 Toast | Non-HTML response, try JSON toast
            try {
                const data = JSON.parse(xhr.responseText)
                if (data.message) {
                    showToast('warning', data.message)
                }
            } catch {
                // 忽略解析错误 | Ignore parse error
            }
            e.detail.shouldSwap = false
        }
    }
})

/* ========================================
   分页滚动管理 | Pagination scroll management
   点击分页链接后自动滚动到表格容器顶部
   Auto-scroll to top of table container after pagination click
   ======================================== */
document.addEventListener('htmx:afterSwap', (e) => {
    const trigger = e.detail.requestConfig && e.detail.requestConfig.elt
    if (!trigger) return

    // 检测分页链接触发 | Detect pagination link trigger
    const isPagination = trigger.closest && trigger.closest('.pagination')
    if (isPagination) {
        // 滚动到目标容器顶部 | Scroll to top of target container
        const target = e.detail.target
        if (target) {
            target.scrollIntoView({ behavior: 'smooth', block: 'start' })
        }
    }
})

/* ========================================
   历史缓存恢复 | History cache restore
   处理浏览器前进/后退时的缓存恢复失败
   Handle cache miss when browser navigates forward/back
   ======================================== */
// 历史缓存未命中时，HTMX 通过 issueAjaxRequest 发起请求，
// htmx:beforeRequest / htmx:afterRequest 已处理进度条，此处无需额外 start()
// On history cache miss, HTMX uses issueAjaxRequest which fires
// htmx:beforeRequest / htmx:afterRequest — progress bar is already handled, no extra start() needed

document.addEventListener('htmx:historyCacheMissError', () => {
    showToast('warning', _isEn() ? 'Failed to restore page, please refresh' : '页面恢复失败，请刷新')
})

