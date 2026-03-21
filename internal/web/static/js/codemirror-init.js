/**
 * codemirror-init.js — CodeMirror 6 初始化封装 | CodeMirror 6 initialization wrapper
 * 提供统一的编辑器/只读查看器创建方法 | Provides unified editor/viewer creation methods
 *
 * 依赖 Dependencies: lib/codemirror6.bundle.min.js (window.CM)
 *
 * 支持语言 Supported languages:
 *   yaml, python, shell/bash, javascript, typescript, xml, lua
 */

/** _cmLangFn 工具类型/语言到 CM6 language 扩展的映射 | Tool type to CM6 language extension mapping */
const _cmLangFn = {
    ansible: () => CM.yaml(),
    yaml: () => CM.yaml(),
    shell: () => CM.shellLang(),
    bash: () => CM.shellLang(),
    slurm: () => CM.shellLang(),
    python: () => CM.python(),
    javascript: () => CM.javascript(),
    typescript: () => CM.javascript({ typescript: true }),
    xml: () => CM.xml(),
    lua: () => CM.luaLang(),
    terraform: () => CM.javascript()           // HCL 近似 | HCL approximation
}

/** _cmPlaceholders 各类型的 placeholder | Placeholder content for each type */
const _cmPlaceholders = {
    ansible: '---\n- hosts: all\n  tasks:\n    - name: Example task\n      debug:\n        msg: "Hello World"',
    shell: '#!/bin/bash\n\n# Your script here\necho "Hello World"',
    bash: '#!/bin/bash\n\n# Your script here\necho "Hello World"',
    terraform: 'resource "example_resource" "name" {\n  # Configuration here\n}',
    slurm: '#!/bin/bash\n#SBATCH --job-name={{.job_name}}\n#SBATCH --output=%j.out\n\necho "Hello from Slurm"',
    python: '#!/usr/bin/env python3\n\nprint("Hello World")',
    lua: '-- Your Lua script here\nprint("Hello World")',
    javascript: '// Your JavaScript here\nconsole.log("Hello World");',
    typescript: '// Your TypeScript here\nconsole.log("Hello World");',
    xml: '<?xml version="1.0" encoding="UTF-8"?>\n<root>\n</root>',
    yaml: '# YAML content here\nkey: value'
}

/** _cmBaseTheme 基础编辑器样式 | Base editor styles */
const _cmBaseTheme = CM.EditorView.theme({
    '&': { height: '100%', fontSize: '13px' },
    '.cm-scroller': { overflow: 'auto', fontFamily: 'var(--font-mono, "Cascadia Code","Fira Code",monospace)' },
    '.cm-gutters': { borderRight: '1px solid var(--color-border, #dee2e6)' },
    '&.cm-focused': { outline: 'none' }
})

/** _cmLightTheme 亮色主题 | Light theme */
const _cmLightTheme = CM.EditorView.theme({
    '&': { backgroundColor: '#fff' },
    '.cm-gutters': { backgroundColor: '#f8f9fa', color: '#6c757d' },
    '.cm-activeLineGutter': { backgroundColor: '#e9ecef' },
    '.cm-activeLine': { backgroundColor: '#f0f4ff' }
})

/** _cmEditorInstances 全局编辑器实例列表（用于主题同步）| Global editor instances for theme sync */
const _cmEditorInstances = []

/**
 * _buildExtensions 构建 CM6 扩展列表 | Build CM6 extension list
 * @param {Object} o 配置项
 * @returns {Array}
 */
function _buildExtensions(o) {
  const langFn = _cmLangFn[o.mode] || _cmLangFn.shell
    const isDark = document.documentElement.getAttribute('data-bs-theme') === 'dark'
  const exts = [
        CM.lineNumbers(),
        CM.highlightActiveLineGutter(),
        CM.history(),
        CM.foldGutter(),
        CM.indentOnInput(),
        CM.bracketMatching(),
        CM.closeBrackets(),
        CM.highlightSelectionMatches(),
        CM.syntaxHighlighting(CM.defaultHighlightStyle, { fallback: true }),
        CM.keymap.of([
            ...CM.closeBracketsKeymap, ...CM.defaultKeymap,
            ...CM.searchKeymap, ...CM.historyKeymap, ...CM.foldKeymap,
            CM.indentWithTab
        ]),
    o.langComp.of(langFn()),
    o.themeComp.of(isDark ? CM.oneDark : _cmLightTheme),
        _cmBaseTheme
    ]
  if (!o.readOnly) {
    exts.push(CM.highlightActiveLine())
    const ph = _cmPlaceholders[o.mode]
    if (ph) exts.push(CM.placeholder(ph))
    }
  if (o.readOnly) {
    exts.push(CM.EditorState.readOnly.of(true), CM.EditorView.editable.of(false))
      exts.push(CM.EditorView.lineWrapping)
    }
  if (o.onChange && !o.readOnly) {
    exts.push(CM.EditorView.updateListener.of(u => {
      if (u.docChanged) o.onChange(u.state.doc.toString())
        }))
    }
  return exts
}

/**
 * createCodeMirror 创建 CodeMirror 6 编辑器 | Create CodeMirror 6 editor
 * @param {HTMLElement} parent  挂载容器
 * @param {Object}  opts
 * @param {string}  opts.mode      语言模式 (ansible|shell|python|yaml|javascript|typescript|xml|lua|terraform|slurm)
 * @param {boolean} opts.readOnly  是否只读
 * @param {string}  opts.value     初始内容
 * @param {Function} opts.onChange 内容变化回调 (value)=>void
 * @returns {{ view: EditorView, langComp: Compartment, themeComp: Compartment, parent: HTMLElement, opts: Object }}
 */
function createCodeMirror(parent, opts = {}) {
  const langComp = new CM.Compartment()
  const themeComp = new CM.Compartment()
  const exts = _buildExtensions({ ...opts, langComp, themeComp })
    const view = new CM.EditorView({
      state: CM.EditorState.create({ doc: opts.value || '', extensions: exts }),
        parent
    })
  const editor = { view, langComp, themeComp, parent, opts }
    _cmEditorInstances.push(editor)
    return editor
}

/**
 * cmSetMode 切换语言模式（销毁旧编辑器并重建）| Switch language mode (destroy and recreate)
 * CM6 的 placeholder ViewPlugin 不支持 Compartment.reconfigure，因此需要重建编辑器
 * CM6's placeholder ViewPlugin does not support Compartment.reconfigure, so we rebuild the editor
 */
function cmSetMode(editor, mode) {
  const doc = editor.view.state.doc.toString()
  const par = editor.parent
  // 从全局实例列表中移除 | Remove from global instance list
  const idx = _cmEditorInstances.indexOf(editor)
  if (idx !== -1) _cmEditorInstances.splice(idx, 1)
  // 销毁旧编辑器 | Destroy old editor
  editor.view.destroy()
  // 用新模式重建 | Rebuild with new mode
  const newOpts = { ...editor.opts, mode, value: doc }
  const langComp = new CM.Compartment()
  const themeComp = new CM.Compartment()
  const exts = _buildExtensions({ ...newOpts, langComp, themeComp })
  const newView = new CM.EditorView({
    state: CM.EditorState.create({ doc: doc, extensions: exts }),
    parent: par
  })
  // 更新 editor 对象的引用 | Update editor object references
  editor.view = newView
  editor.langComp = langComp
  editor.themeComp = themeComp
  editor.opts = newOpts
  _cmEditorInstances.push(editor)
}

/** cmSetTheme 切换编辑器亮/暗主题 | Switch editor light/dark theme */
function cmSetTheme(editor) {
    const isDark = document.documentElement.getAttribute('data-bs-theme') === 'dark'
    editor.view.dispatch({ effects: editor.themeComp.reconfigure(isDark ? CM.oneDark : _cmLightTheme) })
}

/** cmSetValue 设置内容 | Set content */
function cmSetValue(editor, val) {
    editor.view.dispatch({ changes: { from: 0, to: editor.view.state.doc.length, insert: val || '' } })
}

/** cmGetValue 获取内容 | Get content */
function cmGetValue(editor) {
    return editor.view.state.doc.toString()
}

// 监听页面主题变化，自动同步所有 CodeMirror 编辑器主题 | Watch page theme changes, auto-sync all CM editors
if (typeof MutationObserver !== 'undefined') {
    new MutationObserver(mutations => {
        for (const m of mutations) {
            if (m.attributeName === 'data-bs-theme') {
                _cmEditorInstances.forEach(e => {
                    try {
                        cmSetTheme(e)
                    } catch { /* ignore */
                    }
                })
            }
        }
    }).observe(document.documentElement, { attributes: true, attributeFilter: ['data-bs-theme'] })
}

