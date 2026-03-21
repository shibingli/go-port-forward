/**
 * browser-check.js — 浏览器版本检测 | Browser version detection
 * 必须使用 ES5 语法，确保在旧浏览器中也能正常执行
 * MUST use ES5 syntax to ensure execution even in very old browsers
 *
 * 最低支持版本 Minimum supported versions:
 * Chrome 80+ / Firefox 78+ / Edge 79+ / Safari 14.1+
 * ❌ 不支持 IE | DO NOT support IE
 */
;(function() {
    /* 已检测过则跳过（避免多页面重复弹窗）| Skip if already checked (avoid repeated banners) */
    try {
        if (window.sessionStorage && window.sessionStorage.getItem('_bc')) return
    } catch (e) { /* sessionStorage 不可用时继续检测 | Continue if sessionStorage unavailable */
    }

    /**
     * detectBrowser 检测浏览器名称和版本 | Detect browser name and version
     * @returns {{name: string, version: number}}
     */
    function detectBrowser() {
        var ua = navigator.userAgent || ''
        var m

        /* Edge (Chromium) — 必须在 Chrome 之前检测 | Must detect before Chrome */
        m = ua.match(/Edg(?:e|A|iOS)?\/(\d+)/)
        if (m) return { name: 'Edge', version: parseInt(m[1], 10) }

        /* Opera / OPR */
        m = ua.match(/OPR\/(\d+)/)
        if (m) return { name: 'Opera', version: parseInt(m[1], 10) }

        /* Samsung Internet */
        m = ua.match(/SamsungBrowser\/(\d+)/)
        if (m) return { name: 'Samsung', version: parseInt(m[1], 10) }

        /* Firefox */
        m = ua.match(/Firefox\/(\d+)/)
        if (m) return { name: 'Firefox', version: parseInt(m[1], 10) }

        /* Safari — 必须在 Chrome 之前检测（Chrome UA 也包含 Safari）*/
        /* Safari — Must detect before Chrome (Chrome UA also contains Safari) */
        if (/Safari\//.test(ua) && !/Chrome\//.test(ua) && !/Chromium\//.test(ua)) {
            m = ua.match(/Version\/(\d+)\.(\d+)/)
            if (m) return { name: 'Safari', version: parseFloat(m[1] + '.' + m[2]) }
        }

        /* Chrome / Chromium */
        m = ua.match(/(?:Chrome|Chromium)\/(\d+)/)
        if (m) return { name: 'Chrome', version: parseInt(m[1], 10) }

        /* IE — 完全不支持 | Completely unsupported */
        if (/Trident\/|MSIE /.test(ua)) return { name: 'IE', version: 0 }

        return { name: 'Unknown', version: 0 }
    }

    var browser = detectBrowser()

    /* 最低版本要求 | Minimum version requirements */
    var minVersions = {
        Chrome: 80,
        Firefox: 78,
        Edge: 79,
        Safari: 14.1,
        Opera: 67,
        Samsung: 13
    }

    var minVersion = minVersions[browser.name]
    var isSupported = true

    if (browser.name === 'IE') {
        isSupported = false
    } else if (browser.name === 'Unknown') {
        /* 未知浏览器不阻止，但不保证兼容 | Unknown browsers not blocked but not guaranteed */
        isSupported = true
    } else if (minVersion !== undefined && browser.version < minVersion) {
        isSupported = false
    }

    if (isSupported) {
        try {
            if (window.sessionStorage) window.sessionStorage.setItem('_bc', '1')
        } catch (e) { /* ignore */
        }
        return
    }

    /* 构建警告横幅 | Build warning banner */
    var isEn = false
    try {
        var htmlLang = document.documentElement.lang || ''
        isEn = htmlLang.indexOf('en') === 0
    } catch (e) { /* default to Chinese */
    }

    var title = isEn ? 'Browser Not Supported' : '浏览器版本过低'
    var msg = isEn
        ? 'Your browser (' + browser.name + ' ' + browser.version + ') is not supported. Please upgrade to one of the following: Chrome 80+, Firefox 78+, Edge 79+, Safari 14.1+.'
        : '您的浏览器（' + browser.name + ' ' + browser.version + '）版本过低，无法正常使用本系统。请升级至以下浏览器：Chrome 80+、Firefox 78+、Edge 79+、Safari 14.1+。'
    var btnText = isEn ? 'I understand, continue anyway' : '我已了解，继续访问'

    /* 创建横幅 DOM | Create banner DOM */
    var overlay = document.createElement('div')
    overlay.id = 'browser-check-overlay'
    overlay.setAttribute('style',
        'position:fixed;top:0;left:0;width:100%;height:100%;' +
        'background:rgba(0,0,0,0.85);z-index:99999;display:flex;' +
        'align-items:center;justify-content:center;font-family:system-ui,-apple-system,sans-serif;'
    )

    var box = document.createElement('div')
    box.setAttribute('style',
        'background:#fff;border-radius:12px;padding:40px 32px;max-width:520px;' +
        'width:90%;text-align:center;box-shadow:0 8px 32px rgba(0,0,0,0.3);'
    )

    var iconSvg = '<svg width="48" height="48" viewBox="0 0 16 16" fill="#dc3545" xmlns="http://www.w3.org/2000/svg">' +
        '<path d="M8.982 1.566a1.13 1.13 0 0 0-1.96 0L.165 13.233c-.457.778.091 1.767.98 1.767h13.713c.889 0 1.438-.99.98-1.767L8.982 1.566zM8 5c.535 0 .954.462.9.995l-.35 3.507a.552.552 0 0 1-1.1 0L7.1 5.995A.905.905 0 0 1 8 5zm.002 6a1 1 0 1 1 0 2 1 1 0 0 1 0-2z"/>' +
        '</svg>'

    box.innerHTML =
        '<div style="margin-bottom:16px">' + iconSvg + '</div>' +
        '<h2 style="margin:0 0 12px;font-size:22px;color:#212529">' + title + '</h2>' +
        '<p style="margin:0 0 24px;font-size:15px;color:#6c757d;line-height:1.6">' + msg + '</p>' +
        '<button id="browser-check-dismiss" style="' +
        'background:#0d6efd;color:#fff;border:none;border-radius:8px;' +
        'padding:10px 28px;font-size:15px;cursor:pointer;transition:background 0.2s' +
        '">' + btnText + '</button>'

    overlay.appendChild(box)

    /* 等待 DOM 就绪后插入 | Insert after DOM is ready */
    function insertBanner() {
        document.body.appendChild(overlay)
        var btn = document.getElementById('browser-check-dismiss')
        if (btn) {
            btn.onclick = function() {
                overlay.parentNode.removeChild(overlay)
                try {
                    if (window.sessionStorage) window.sessionStorage.setItem('_bc', '1')
                } catch (e) { /* ignore */
                }
            }
        }
    }

    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', insertBanner)
    } else {
        insertBanner()
    }
})()

