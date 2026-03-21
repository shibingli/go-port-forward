/**
 * user-preferences.js — 用户偏好设置管理 | User preferences management
 * Alpine.js 数据组件：主题切换、主色调、信息密度、侧边栏、语言、每页条数
 * Alpine.js data component: theme, primary color, density, sidebar, language, page size
 *
 * 依赖 Dependencies: Alpine.js 3.15.8+, app.js (showToast)
 */

'use strict'

/* ========================================
   颜色工具函数已移至 app.js（hexToRgb, adjustColor, applyPrimaryColor）
   Color utility functions moved to app.js (hexToRgb, adjustColor, applyPrimaryColor)
   ======================================== */

/* ========================================
   Alpine.js 偏好管理组件 | Alpine.js preferences manager component
   ======================================== */

/**
 * preferencesManager Alpine.js 数据组件 | Alpine.js data component
 * 用于个人设置页面的偏好管理 For preferences page management
 * @param {Object} initial - 初始偏好值（从服务端注入）Initial preferences (injected from server)
 * @returns {Object} Alpine.js 数据对象 Alpine.js data object
 */
const preferencesManager = (initial = {}) => ({
    // 偏好数据 | Preference data
    themeMode: initial.theme_mode || 'light',
    primaryColor: initial.primary_color || '',
    sidebarCollapsed: initial.sidebar_collapsed || false,
    density: initial.density || 'default',
    pageSize: initial.page_size || 20,
    locale: initial.locale || 'zh-CN',
    dashboardLayout: initial.dashboard_layout || 'default',

    // 状态 | State
    saving: false,
    dirty: false,

    /** init 初始化组件 | Initialize component */
    init() {
        this.$watch('themeMode', () => {
            this.applyTheme()
            this.dirty = true
        })
        this.$watch('primaryColor', () => {
            this.applyPrimaryColor()
            this.dirty = true
        })
        this.$watch('density', () => {
            this.applyDensity()
            this.dirty = true
        })
        this.$watch('sidebarCollapsed', () => {
            this.dirty = true
        })
        this.$watch('pageSize', () => {
            this.dirty = true
        })
        this.$watch('dashboardLayout', () => {
            this.dirty = true
        })
    },

    /** applyTheme 实时预览主题模式 | Real-time theme mode preview */
    applyTheme() {
        const html = document.documentElement
        if (this.themeMode === 'auto') {
            const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches
            html.setAttribute('data-bs-theme', prefersDark ? 'dark' : 'light')
            html.setAttribute('data-bs-theme-mode', 'auto')
        } else {
            html.setAttribute('data-bs-theme', this.themeMode)
            html.removeAttribute('data-bs-theme-mode')
        }
    },

    /** applyPrimaryColor 实时预览自定义主色调 | Real-time custom primary color preview */
    // 使用 app.js 中的全局 applyPrimaryColor 函数 | Uses global applyPrimaryColor from app.js
    applyPrimaryColor() {
        applyPrimaryColor(this.primaryColor)
    },

    /** applyDensity 实时预览信息密度 | Real-time information density preview */
    applyDensity() {
        const html = document.documentElement
        html.classList.remove('density-compact', 'density-comfortable')
        if (this.density !== 'default') {
            html.classList.add(`density-${this.density}`)
        }
    },

    /** switchLocale 切换语言 | Switch language */
    switchLocale(newLocale) {
        this.locale = newLocale
        // 设置 Cookie（服务端需要读取语言偏好）| Set cookie (server needs to read language preference)
        SecureStore.setCookie('hpc_lang', newLocale, { maxAge: 365 * 24 * 3600 })
        // 保存后刷新页面 | Save then refresh page
        this.save(() => {
            window.location.reload()
        })
    },

    /** save 保存偏好到服务器 | Save preferences to server */
    save(onSuccess) {
        if (this.saving) return
        this.saving = true

        const data = {
            theme_mode: this.themeMode,
            primary_color: this.primaryColor,
            sidebar_collapsed: this.sidebarCollapsed,
            density: this.density,
            page_size: this.pageSize,
            dashboard_layout: this.dashboardLayout,
            locale: this.locale
        }

        apiPatch('/api/v1/users/me/preferences', data)
            .then((res) => {
                if (res.code !== 0) throw new Error(res.message || 'Save failed')
                this.dirty = false
                const isEn = document.documentElement.lang === 'en'
                showToast('success', isEn ? 'Preferences saved' : '偏好设置已保存')
                if (typeof onSuccess === 'function') onSuccess()
            })
            .catch(() => {
                const isEn = document.documentElement.lang === 'en'
                showToast('danger', isEn ? 'Failed to save preferences' : '保存偏好设置失败')
            })
            .finally(() => {
                this.saving = false
            })
    },

    /** resetToDefaults 重置为默认偏好 | Reset to default preferences */
    resetToDefaults() {
        showConfirm(
            document.documentElement.lang === 'en' ? 'Reset all preferences to defaults?' : '确定重置所有偏好设置为默认值？',
            () => {
                apiDelete('/api/v1/users/me/preferences')
                    .then((res) => {
                        if (res.code !== 0) throw new Error(res.message || 'Reset failed')
                        window.location.reload()
                    })
                    .catch(() => {
                        const isEn = document.documentElement.lang === 'en'
                        showToast('danger', isEn ? 'Failed to reset preferences' : '重置偏好设置失败')
                    })
            },
            'warning'
        )
    }
})

// 注册 Alpine.js 组件 | Register Alpine.js component
document.addEventListener('alpine:init', () => {
    Alpine.data('preferencesManager', preferencesManager)
})

