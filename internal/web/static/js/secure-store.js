/**
 * secure-store.js — 前端统一安全存储模块 | Unified secure storage module
 * 默认使用 IndexedDB 存储，不支持时降级为 localStorage
 * Uses IndexedDB by default, falls back to localStorage when unsupported
 *
 * 功能 Features:
 * - IndexedDB 主存储 + localStorage 降级 | IndexedDB primary + localStorage fallback
 * - AES-GCM 加密（Web Crypto API）+ 同步 XOR 混淆降级 | AES-GCM encryption + sync XOR obfuscation fallback
 * - LZW 压缩/解压 | LZW compression/decompression
 * - 同步内存缓存层（支持 FOUC 防闪烁等同步读取场景）| Sync in-memory cache layer
 * - Cookie 辅助方法（服务端通信用）| Cookie helpers for server-side communication
 *
 * 依赖 Dependencies: 无 None (vanilla ES6+)
 * 加载顺序 Load order: 必须在 app.js 之前加载 MUST load before app.js
 */

'use strict'

;(function(window) {
    /* ========================================
       配置常量 | Configuration constants
       ======================================== */
    const DB_NAME = 'hpc_secure_store'
    const DB_VERSION = 1
    const STORE_NAME = 'kv'
    const LS_PREFIX = '_ss_'
    const KEY_STORAGE = '_ss_ek'
    const SYNC_KEY_STORAGE = '_ss_sk'

    /* ========================================
       内部状态 | Internal state
       ======================================== */
    let _cache = new Map()
    let _db = null
    let _cryptoKey = null
    let _syncKey = null
    let _useIDB = false
    let _useCrypto = false
    let _initialized = false

    /* ========================================
       LZW 压缩/解压 | LZW compression/decompression
       ======================================== */

    /**
     * _compress LZW 压缩 | LZW compress
     * @param {string} str - 原始字符串 Original string
     * @returns {string} 压缩后的字符串（UTF-16 编码）Compressed string (UTF-16 encoded)
     */
    const _compress = (str) => {
        if (!str) return ''
        const dict = new Map()
        let dictSize = 256
        for (let i = 0; i < 256; i++) dict.set(String.fromCharCode(i), i)
        let w = ''
        const result = []
        for (let i = 0; i < str.length; i++) {
            const c = str[i]
            const wc = w + c
            if (dict.has(wc)) {
                w = wc
            } else {
                result.push(dict.get(w))
                dict.set(wc, dictSize++)
                w = c
            }
        }
        if (w) result.push(dict.get(w))
        return result.map(code => String.fromCharCode(code)).join('')
    }

    /**
     * _decompress LZW 解压 | LZW decompress
     * @param {string} str - 压缩字符串 Compressed string
     * @returns {string} 原始字符串 Original string
     */
    const _decompress = (str) => {
        if (!str) return ''
        const dict = new Map()
        let dictSize = 256
        for (let i = 0; i < 256; i++) dict.set(i, String.fromCharCode(i))
        const codes = []
        for (let i = 0; i < str.length; i++) codes.push(str.charCodeAt(i))
        let w = String.fromCharCode(codes[0])
        const result = [w]
        for (let i = 1; i < codes.length; i++) {
            const k = codes[i]
            let entry
            if (dict.has(k)) {
                entry = dict.get(k)
            } else if (k === dictSize) {
                entry = w + w[0]
            } else {
                throw new Error('LZW: invalid code')
            }
            result.push(entry)
            dict.set(dictSize++, w + entry[0])
            w = entry
        }
        return result.join('')
    }

    /* ========================================
       同步加密/解密（XOR 混淆）| Sync encryption/decryption (XOR obfuscation)
       用于 localStorage 同步镜像 | For localStorage sync mirror
       ======================================== */

    /**
     * _getSyncKey 获取或生成同步加密密钥 | Get or generate sync encryption key
     * @returns {Uint8Array} 32 字节密钥 32-byte key
     */
    const _getSyncKey = () => {
        if (_syncKey) return _syncKey
        try {
            const stored = localStorage.getItem(SYNC_KEY_STORAGE)
            if (stored) {
                _syncKey = new Uint8Array(atob(stored).split('').map(c => c.charCodeAt(0)))
                return _syncKey
            }
        } catch { /* 忽略 Ignore */
        }
        // 生成新密钥 | Generate new key
        _syncKey = new Uint8Array(32)
        if (window.crypto && window.crypto.getRandomValues) {
            window.crypto.getRandomValues(_syncKey)
        } else {
            for (let i = 0; i < 32; i++) _syncKey[i] = Math.floor(Math.random() * 256)
        }
        try {
            localStorage.setItem(SYNC_KEY_STORAGE, btoa(String.fromCharCode(..._syncKey)))
        } catch { /* 忽略 Ignore */
        }
        return _syncKey
    }

    /**
     * _encryptSync 同步 XOR 加密 | Sync XOR encrypt
     * @param {string} str - 明文 Plaintext
     * @returns {string} Base64 编码的密文 Base64-encoded ciphertext
     */
    const _encryptSync = (str) => {
        const key = _getSyncKey()
        const bytes = new TextEncoder().encode(str)
        const encrypted = new Uint8Array(bytes.length)
        for (let i = 0; i < bytes.length; i++) {
            encrypted[i] = bytes[i] ^ key[i % key.length]
        }
        return btoa(String.fromCharCode(...encrypted))
    }


    /**
     * _decryptSync 同步 XOR 解密 | Sync XOR decrypt
     * @param {string} b64 - Base64 编码的密文 Base64-encoded ciphertext
     * @returns {string} 明文 Plaintext
     */
    const _decryptSync = (b64) => {
        const key = _getSyncKey()
        const bytes = new Uint8Array(atob(b64).split('').map(c => c.charCodeAt(0)))
        const decrypted = new Uint8Array(bytes.length)
        for (let i = 0; i < bytes.length; i++) {
            decrypted[i] = bytes[i] ^ key[i % key.length]
        }
        return new TextDecoder().decode(decrypted)
    }

    /* ========================================
       异步加密/解密（AES-GCM）| Async encryption/decryption (AES-GCM)
       用于 IndexedDB 主存储 | For IndexedDB primary storage
       ======================================== */

    /**
     * _generateCryptoKey 生成或加载 AES-GCM 密钥 | Generate or load AES-GCM key
     * @returns {Promise<void>}
     */
    const _generateCryptoKey = async () => {
        if (!_useCrypto) return
        try {
            const stored = localStorage.getItem(KEY_STORAGE)
            if (stored) {
                const raw = Uint8Array.from(atob(stored), c => c.charCodeAt(0))
                _cryptoKey = await window.crypto.subtle.importKey(
                    'raw', raw, { name: 'AES-GCM' }, false, ['encrypt', 'decrypt']
                )
                return
            }
            // 生成新密钥 | Generate new key
            _cryptoKey = await window.crypto.subtle.generateKey(
                { name: 'AES-GCM', length: 256 }, true, ['encrypt', 'decrypt']
            )
            const exported = await window.crypto.subtle.exportKey('raw', _cryptoKey)
            localStorage.setItem(KEY_STORAGE, btoa(String.fromCharCode(...new Uint8Array(exported))))
        } catch {
            _useCrypto = false
        }
    }

    /**
     * _encryptAsync AES-GCM 异步加密 | AES-GCM async encrypt
     * @param {string} str - 明文 Plaintext
     * @returns {Promise<string>} Base64 编码的 IV+密文 Base64-encoded IV+ciphertext
     */
    const _encryptAsync = async (str) => {
        if (!_useCrypto || !_cryptoKey) return _encryptSync(str)
        try {
            const iv = window.crypto.getRandomValues(new Uint8Array(12))
            const encoded = new TextEncoder().encode(str)
            const encrypted = await window.crypto.subtle.encrypt(
                { name: 'AES-GCM', iv }, _cryptoKey, encoded
            )
            // IV(12) + ciphertext 拼接后 Base64 | Concatenate IV(12) + ciphertext then Base64
            const combined = new Uint8Array(iv.length + encrypted.byteLength)
            combined.set(iv)
            combined.set(new Uint8Array(encrypted), iv.length)
            return btoa(String.fromCharCode(...combined))
        } catch {
            return _encryptSync(str)
        }
    }

    /**
     * _decryptAsync AES-GCM 异步解密 | AES-GCM async decrypt
     * @param {string} b64 - Base64 编码的 IV+密文 Base64-encoded IV+ciphertext
     * @returns {Promise<string>} 明文 Plaintext
     */
    const _decryptAsync = async (b64) => {
        if (!_useCrypto || !_cryptoKey) return _decryptSync(b64)
        try {
            const combined = Uint8Array.from(atob(b64), c => c.charCodeAt(0))
            const iv = combined.slice(0, 12)
            const data = combined.slice(12)
            const decrypted = await window.crypto.subtle.decrypt(
                { name: 'AES-GCM', iv }, _cryptoKey, data
            )
            return new TextDecoder().decode(decrypted)
        } catch {
            // 降级尝试同步解密（兼容旧数据）| Fallback to sync decrypt (compatible with old data)
            try {
                return _decryptSync(b64)
            } catch {
                return ''
            }
        }
    }

    /* ========================================
       IndexedDB 操作 | IndexedDB operations
       ======================================== */

    /**
     * _openDB 打开 IndexedDB 数据库 | Open IndexedDB database
     * @returns {Promise<IDBDatabase>}
     */
    const _openDB = () => new Promise((resolve, reject) => {
        if (!_useIDB) {
            reject(new Error('IndexedDB not available'))
            return
        }
        const req = indexedDB.open(DB_NAME, DB_VERSION)
        req.onupgradeneeded = (e) => {
            const db = e.target.result
            if (!db.objectStoreNames.contains(STORE_NAME)) {
                db.createObjectStore(STORE_NAME)
            }
        }
        req.onsuccess = (e) => resolve(e.target.result)
        req.onerror = () => reject(req.error)
    })

    /**
     * _idbGet 从 IndexedDB 读取 | Read from IndexedDB
     * @param {string} key - 键名 Key name
     * @returns {Promise<string|null>}
     */
    const _idbGet = (key) => new Promise((resolve, reject) => {
        if (!_db) {
            resolve(null)
            return
        }
        const tx = _db.transaction(STORE_NAME, 'readonly')
        const req = tx.objectStore(STORE_NAME).get(key)
        req.onsuccess = () => resolve(req.result ?? null)
        req.onerror = () => reject(req.error)
    })

    /**
     * _idbSet 写入 IndexedDB | Write to IndexedDB
     * @param {string} key - 键名 Key name
     * @param {string} value - 值 Value
     * @returns {Promise<void>}
     */
    const _idbSet = (key, value) => new Promise((resolve, reject) => {
        if (!_db) {
            resolve()
            return
        }
        const tx = _db.transaction(STORE_NAME, 'readwrite')
        const req = tx.objectStore(STORE_NAME).put(value, key)
        req.onsuccess = () => resolve()
        req.onerror = () => reject(req.error)
    })

    /**
     * _idbRemove 从 IndexedDB 删除 | Remove from IndexedDB
     * @param {string} key - 键名 Key name
     * @returns {Promise<void>}
     */
    const _idbRemove = (key) => new Promise((resolve, reject) => {
        if (!_db) {
            resolve()
            return
        }
        const tx = _db.transaction(STORE_NAME, 'readwrite')
        const req = tx.objectStore(STORE_NAME).delete(key)
        req.onsuccess = () => resolve()
        req.onerror = () => reject(req.error)
    })

    /**
     * _idbClear 清空 IndexedDB | Clear IndexedDB
     * @returns {Promise<void>}
     */
    const _idbClear = () => new Promise((resolve, reject) => {
        if (!_db) {
            resolve()
            return
        }
        const tx = _db.transaction(STORE_NAME, 'readwrite')
        const req = tx.objectStore(STORE_NAME).clear()
        req.onsuccess = () => resolve()
        req.onerror = () => reject(req.error)
    })


    /* ========================================
       localStorage 同步镜像 | localStorage sync mirror
       ======================================== */

    /**
     * _lsSet 写入 localStorage 同步镜像 | Write to localStorage sync mirror
     * @param {string} key - 键名 Key name
     * @param {string} compressed - 压缩后的字符串 Compressed string
     */
    const _lsSet = (key, compressed) => {
        try {
            localStorage.setItem(LS_PREFIX + key, _encryptSync(compressed))
        } catch { /* 存储满或不可用 Storage full or unavailable */
        }
    }

    /**
     * _lsGet 从 localStorage 同步镜像读取 | Read from localStorage sync mirror
     * @param {string} key - 键名 Key name
     * @returns {string|null} 压缩后的字符串或 null Compressed string or null
     */
    const _lsGet = (key) => {
        try {
            const val = localStorage.getItem(LS_PREFIX + key)
            if (val === null) return null
            return _decryptSync(val)
        } catch {
            return null
        }
    }

    /**
     * _lsRemove 从 localStorage 同步镜像删除 | Remove from localStorage sync mirror
     * @param {string} key - 键名 Key name
     */
    const _lsRemove = (key) => {
        try {
            localStorage.removeItem(LS_PREFIX + key)
        } catch { /* 忽略 Ignore */
        }
    }

    /**
     * _loadSyncCache 从 localStorage 同步镜像加载缓存 | Load cache from localStorage sync mirror
     * 在脚本加载时同步执行，填充内存缓存 | Executed synchronously on script load, populates memory cache
     */
    const _loadSyncCache = () => {
        try {
            for (let i = 0; i < localStorage.length; i++) {
                const fullKey = localStorage.key(i)
                if (!fullKey || !fullKey.startsWith(LS_PREFIX)) continue
                const key = fullKey.slice(LS_PREFIX.length)
                // 跳过内部密钥存储 | Skip internal key storage
                if (fullKey === KEY_STORAGE || fullKey === SYNC_KEY_STORAGE) continue
                try {
                    const compressed = _decryptSync(localStorage.getItem(fullKey))
                    const value = _decompress(compressed)
                    _cache.set(key, JSON.parse(value))
                } catch { /* 跳过损坏的条目 Skip corrupted entries */
                }
            }
        } catch { /* localStorage 不可用 localStorage unavailable */
        }
    }

    /* ========================================
       公共 API | Public API
       ======================================== */

    const SecureStore = {
        /**
         * init 异步初始化 | Async initialization
         * 打开 IndexedDB，生成加密密钥，从 IndexedDB 加载数据到缓存
         * Opens IndexedDB, generates crypto key, loads data from IndexedDB to cache
         * @returns {Promise<void>}
         */
        async init() {
            if (_initialized) return
            // 检测 IndexedDB 可用性 | Detect IndexedDB availability
            _useIDB = !!window.indexedDB
            // 检测 Web Crypto API 可用性 | Detect Web Crypto API availability
            _useCrypto = !!(window.crypto && window.crypto.subtle)

            // 生成/加载加密密钥 | Generate/load encryption key
            await _generateCryptoKey()

            // 打开 IndexedDB | Open IndexedDB
            if (_useIDB) {
                try {
                    _db = await _openDB()
                    // 从 IndexedDB 加载所有数据到缓存 | Load all data from IndexedDB to cache
                    await _loadFromIDB()
                } catch {
                    _useIDB = false
                    _db = null
                }
            }
            _initialized = true
        },

        /**
         * getSync 同步读取 | Synchronous read
         * 从内存缓存中读取数据（脚本加载时已从 localStorage 填充）
         * Reads from in-memory cache (populated from localStorage on script load)
         * @param {string} key - 键名 Key name
         * @returns {*} 值或 null Value or null
         */
        getSync(key) {
            return _cache.has(key) ? _cache.get(key) : null
        },

        /**
         * get 异步读取 | Async read
         * 优先从缓存读取，缓存未命中时从存储读取
         * Reads from cache first, falls back to storage on cache miss
         * @param {string} key - 键名 Key name
         * @returns {Promise<*>} 值或 null Value or null
         */
        async get(key) {
            if (_cache.has(key)) return _cache.get(key)
            // 尝试从 IndexedDB 读取 | Try reading from IndexedDB
            if (_db) {
                try {
                    const encrypted = await _idbGet(key)
                    if (encrypted !== null) {
                        const compressed = await _decryptAsync(encrypted)
                        const value = JSON.parse(_decompress(compressed))
                        _cache.set(key, value)
                        return value
                    }
                } catch { /* 忽略 Ignore */
                }
            }
            return null
        },

        /**
         * set 异步写入 | Async write
         * 同时写入内存缓存、IndexedDB 和 localStorage 同步镜像
         * Writes to memory cache, IndexedDB, and localStorage sync mirror
         * @param {string} key - 键名 Key name
         * @param {*} value - 值（将被 JSON 序列化）Value (will be JSON serialized)
         * @returns {Promise<void>}
         */
        async set(key, value) {
            _cache.set(key, value)
            const json = JSON.stringify(value)
            const compressed = _compress(json)
            // 写入 localStorage 同步镜像（同步加密）| Write to localStorage sync mirror (sync encrypt)
            _lsSet(key, compressed)
            // 写入 IndexedDB（异步加密）| Write to IndexedDB (async encrypt)
            if (_db) {
                try {
                    const encrypted = await _encryptAsync(compressed)
                    await _idbSet(key, encrypted)
                } catch { /* 忽略 Ignore */
                }
            }
        },

        /**
         * remove 异步删除 | Async remove
         * 从内存缓存、IndexedDB 和 localStorage 同步镜像中删除
         * Removes from memory cache, IndexedDB, and localStorage sync mirror
         * @param {string} key - 键名 Key name
         * @returns {Promise<void>}
         */
        async remove(key) {
            _cache.delete(key)
            _lsRemove(key)
            if (_db) {
                try {
                    await _idbRemove(key)
                } catch { /* 忽略 Ignore */
                }
            }
        },

        /**
         * clear 异步清空所有数据 | Async clear all data
         * 清空内存缓存、IndexedDB 和 localStorage 同步镜像（保留加密密钥）
         * Clears memory cache, IndexedDB, and localStorage sync mirror (preserves encryption keys)
         * @returns {Promise<void>}
         */
        async clear() {
            _cache.clear()
            // 清除 localStorage 中的数据条目（保留密钥）| Clear data entries in localStorage (preserve keys)
            try {
                const keysToRemove = []
                for (let i = 0; i < localStorage.length; i++) {
                    const k = localStorage.key(i)
                    if (k && k.startsWith(LS_PREFIX) && k !== KEY_STORAGE && k !== SYNC_KEY_STORAGE) {
                        keysToRemove.push(k)
                    }
                }
                keysToRemove.forEach(k => localStorage.removeItem(k))
            } catch { /* 忽略 Ignore */
            }
            // 清空 IndexedDB | Clear IndexedDB
            if (_db) {
                try {
                    await _idbClear()
                } catch { /* 忽略 Ignore */
                }
            }
        },

        /**
         * setCookie 设置 Cookie | Set cookie
         * 用于需要服务端读取的数据（如语言偏好、认证令牌）
         * For data that needs to be read server-side (e.g., language preference, auth token)
         * @param {string} name - Cookie 名称 Cookie name
         * @param {string} value - Cookie 值 Cookie value
         * @param {Object} [opts] - 选项 Options
         * @param {number} [opts.maxAge] - 最大存活秒数 Max age in seconds
         * @param {string} [opts.path='/'] - 路径 Path
         * @param {string} [opts.sameSite='Lax'] - SameSite 策略 SameSite policy
         */
        setCookie(name, value, opts = {}) {
            const path = opts.path || '/'
            const sameSite = opts.sameSite || 'Lax'
            let cookie = `${encodeURIComponent(name)}=${encodeURIComponent(value)};path=${path};SameSite=${sameSite}`
            if (opts.maxAge !== undefined) cookie += `;max-age=${opts.maxAge}`
            document.cookie = cookie
        },

        /**
         * removeCookie 删除 Cookie | Remove cookie
         * @param {string} name - Cookie 名称 Cookie name
         * @param {string} [path='/'] - 路径 Path
         */
        removeCookie(name, path = '/') {
            document.cookie = `${encodeURIComponent(name)}=;path=${path};expires=Thu, 01 Jan 1970 00:00:00 GMT`
        },

        /**
         * isInitialized 检查是否已初始化 | Check if initialized
         * @returns {boolean}
         */
        isInitialized() {
            return _initialized
        },

        /**
         * getStorageType 获取当前存储类型 | Get current storage type
         * @returns {string} 'indexeddb' 或 'localstorage' | 'indexeddb' or 'localstorage'
         */
        getStorageType() {
            return _db ? 'indexeddb' : 'localstorage'
        },

        /**
         * getEncryptionType 获取当前加密类型 | Get current encryption type
         * @returns {string} 'aes-gcm' 或 'xor' | 'aes-gcm' or 'xor'
         */
        getEncryptionType() {
            return _useCrypto && _cryptoKey ? 'aes-gcm' : 'xor'
        }
    }

    /**
     * _loadFromIDB 从 IndexedDB 加载所有数据到缓存 | Load all data from IndexedDB to cache
     * @returns {Promise<void>}
     */
    const _loadFromIDB = () => new Promise((resolve, reject) => {
        if (!_db) {
            resolve()
            return
        }
        const tx = _db.transaction(STORE_NAME, 'readonly')
        const store = tx.objectStore(STORE_NAME)
        const req = store.openCursor()
        req.onsuccess = async (e) => {
            const cursor = e.target.result
            if (cursor) {
                try {
                    const compressed = await _decryptAsync(cursor.value)
                    const value = JSON.parse(_decompress(compressed))
                    _cache.set(cursor.key, value)
                } catch { /* 跳过损坏的条目 Skip corrupted entries */
                }
                cursor.continue()
            } else {
                resolve()
            }
        }
        req.onerror = () => reject(req.error)
    })

    /* ========================================
       脚本加载时同步初始化 | Synchronous initialization on script load
       ======================================== */

    // 从 localStorage 同步镜像加载缓存（同步，用于 FOUC 防闪烁等场景）
    // Load cache from localStorage sync mirror (synchronous, for FOUC prevention etc.)
    _loadSyncCache()

    // 挂载到全局 | Mount to global
    window.SecureStore = SecureStore

})(window)