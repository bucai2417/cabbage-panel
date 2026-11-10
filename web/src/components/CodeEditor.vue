<script setup>
import { ref, nextTick, onMounted, onBeforeUnmount, watch } from 'vue'
import { basicSetup } from 'codemirror'
import { EditorView, keymap, Decoration, ViewPlugin } from '@codemirror/view'
import { EditorState, Compartment, Prec } from '@codemirror/state'
import { indentWithTab, undo, redo, isolateHistory } from '@codemirror/commands'
import { indentUnit, StreamLanguage } from '@codemirror/language'
import { javascript } from '@codemirror/lang-javascript'
import { python } from '@codemirror/lang-python'
import { json, jsonParseLinter } from '@codemirror/lang-json'
import { shell } from '@codemirror/legacy-modes/mode/shell'
import { autocompletion, completeFromList, snippetCompletion, startCompletion } from '@codemirror/autocomplete'
import { SearchQuery, setSearchQuery, findNext, findPrevious, replaceNext, replaceAll, search, gotoLine } from '@codemirror/search'
import { linter, lintGutter, setDiagnostics } from '@codemirror/lint'
import Icon from './Icon.vue'
const props = defineProps({ modelValue: { type: String, default: '' }, filename: String, readonly: Boolean })
const emit = defineEmits(['update:modelValue', 'save'])
const host = ref(null), position = ref('1:1'), wrap = ref(false)
const searchOpen = ref(false), replaceOpen = ref(false), searchText = ref(''), replaceText = ref('')
const caseSensitive = ref(false), wholeWord = ref(false), useRegex = ref(false), matchText = ref('')
const searchInput = ref(null)
const wrapping = new Compartment()
let view
const pythonWords = ['print', 'len', 'range', 'str', 'int', 'float', 'list', 'dict', 'set', 'tuple', 'enumerate', 'zip', 'open', 'sorted', 'isinstance', 'input', 'sum', 'min', 'max', 'abs', 'any', 'all', 'map', 'filter', 'super', 'Exception', 'ValueError', 'True', 'False', 'None', 'async', 'await', 'import', 'from', 'return', 'yield', 'with', 'try', 'except', 'finally']
const shellWords = ['echo', 'printf', 'export', 'source', 'cd', 'pwd', 'read', 'test', 'exit', 'return', 'local', 'set', 'if', 'then', 'else', 'fi', 'for', 'do', 'done', 'while', 'case', 'esac', 'task', 'python3', 'node', 'curl', 'grep', 'mkdir', 'cp', 'mv']
function localWords(context) {
  const match = context.matchBefore(/[\w$]+/)
  if (!match || (!context.explicit && !match.text)) return null
  const words = [...new Set(context.state.doc.toString().match(/[A-Za-z_$][\w$]{2,}/g) || [])]
  return { from: match.from, options: words.filter(word => word !== match.text).slice(0, 800).map(label => ({ label, type: 'variable', detail: '当前文件' })), validFor: /^[\w$]*$/ }
}
function language() {
  const extension = props.filename?.split('.').pop()?.toLowerCase()
  if (extension === 'json') return [json(), linter(jsonParseLinter())]
  if (extension === 'py') return [python(), python().language.data.of({ autocomplete: completeFromList([
    ...pythonWords.map(label => ({ label, type: 'keyword' })),
    snippetCompletion('def \${name}(\${args}):\n    \${pass}', { label: 'def', type: 'function', detail: '定义函数' }),
    snippetCompletion('for \${item} in \${items}:\n    \${pass}', { label: 'for', type: 'keyword', detail: '循环模板' }),
    snippetCompletion('if __name__ == "__main__":\n    \${main()}', { label: 'main', type: 'keyword', detail: '程序入口' }),
  ]) })]
  if (['sh', 'bash'].includes(extension)) { const support = StreamLanguage.define(shell); return [support, support.data.of({ autocomplete: completeFromList(shellWords.map(label => ({ label, type: 'keyword' }))) })] }
  return [javascript({ typescript: extension === 'ts' })]
}
function guides(editor) {
  const ranges = []
  for (const visible of editor.visibleRanges) {
    for (let offset = visible.from; offset <= visible.to;) {
      const line = editor.state.doc.lineAt(offset), whitespace = line.text.match(/^\s+/)?.[0]
      if (whitespace?.length) ranges.push(Decoration.mark({ class: 'cm-indent-guide' }).range(line.from, line.from + whitespace.length))
      if (line.to >= visible.to) break
      offset = line.to + 1
    }
  }
  return Decoration.set(ranges, true)
}
const indentationGuides = ViewPlugin.fromClass(class {
  constructor(editor) { this.decorations = guides(editor) }
  update(update) { if (update.docChanged || update.viewportChanged) this.decorations = guides(update.view) }
}, { decorations: plugin => plugin.decorations })
function searchQuery() {
  return new SearchQuery({ search: searchText.value, caseSensitive: caseSensitive.value, wholeWord: wholeWord.value, regexp: useRegex.value, replace: replaceText.value })
}
function applyQuery() {
  if (!view) return
  view.dispatch({ effects: setSearchQuery.of(searchQuery()) })
  countMatches()
}
function countMatches() {
  if (!view) return
  const query = searchQuery()
  if (!query.search) { matchText.value = ''; return }
  if (!query.valid) { matchText.value = '正则无效'; return }
  const selection = view.state.selection.main
  const cursor = query.getCursor(view.state)
  let total = 0, current = 0
  for (let step = cursor.next(); !step.done; step = cursor.next()) {
    total++
    if (step.value.from >= selection.from && step.value.to <= selection.to) current = total
  }
  matchText.value = total ? (current || 1) + '/' + total : '无结果'
}
function openSearch(withReplace = false) {
  if (!view) return
  const selected = view.state.sliceDoc(view.state.selection.main.from, view.state.selection.main.to)
  if (selected && selected.length < 200 && !selected.includes('\n')) searchText.value = selected
  searchOpen.value = true
  if (withReplace) replaceOpen.value = true
  nextTick(() => { searchInput.value?.focus(); searchInput.value?.select(); applyQuery() })
}
function closeSearch() {
  searchOpen.value = false
  replaceOpen.value = false
  if (view) { view.dispatch({ effects: setSearchQuery.of(new SearchQuery({ search: '' })) }); view.focus() }
}
function searchNext() {
  if (!view) return
  applyQuery()
  if (!searchText.value || !searchQuery().valid) return
  findNext(view)
  countMatches()
}
function searchPrev() {
  if (!view) return
  applyQuery()
  if (!searchText.value || !searchQuery().valid) return
  findPrevious(view)
  countMatches()
}
function replaceOne() {
  if (!view) return
  applyQuery()
  if (!searchText.value || !searchQuery().valid) return
  replaceNext(view)
  countMatches()
}
function replaceAllMatches() {
  if (!view) return
  applyQuery()
  if (!searchText.value || !searchQuery().valid) return
  replaceAll(view)
  countMatches()
}
function toggleOption(key) {
  if (key === 'caseSensitive') caseSensitive.value = !caseSensitive.value
  if (key === 'wholeWord') wholeWord.value = !wholeWord.value
  if (key === 'useRegex') useRegex.value = !useRegex.value
  applyQuery()
}
function onSearchEnter(event) { event.shiftKey ? searchPrev() : searchNext() }
function reopenSearch() { searchInput.value?.focus(); searchInput.value?.select() }
onMounted(() => {
  view = new EditorView({ parent: host.value, state: EditorState.create({ doc: props.modelValue, extensions: [
    basicSetup, search(), lintGutter(), EditorState.transactionExtender.of(transaction => transaction.isUserEvent('input.indent') || transaction.isUserEvent('delete.dedent') ? { annotations: isolateHistory.of('full') } : null), language(), indentUnit.of('    '), EditorState.tabSize.of(4), indentationGuides,
    EditorView.contentAttributes.of({ 'aria-label': '文件内容编辑器', spellcheck: 'false' }),
    EditorState.readOnly.of(props.readonly), EditorView.editable.of(!props.readonly), wrapping.of([]),
    Prec.highest(keymap.of([
      { key: 'Mod-f', run: () => { openSearch(false); return true }, preventDefault: true },
      { key: 'Mod-h', run: () => { openSearch(true); return true }, preventDefault: true },
      { key: 'F3', run: () => { searchNext(); return true } },
      { key: 'Shift-F3', run: () => { searchPrev(); return true } },
      indentWithTab,
      { key: 'Mod-Shift-z', run: redo, preventDefault: true },
      { key: 'Mod-y', run: redo, preventDefault: true },
      { key: 'Mod-s', run: () => { emit('save'); return true } },
    ])),
    autocompletion(), EditorState.languageData.of(() => [{ autocomplete: localWords }]),
    EditorView.updateListener.of(update => {
      if (update.docChanged) emit('update:modelValue', update.state.doc.toString())
      if ((update.docChanged || update.selectionSet) && searchOpen.value) countMatches()
      const cursor = update.state.selection.main.head, line = update.state.doc.lineAt(cursor)
      position.value = line.number + ':' + (cursor - line.from + 1)
    }),
    EditorView.theme({ '&': { height: '100%', fontSize: '13px' }, '.cm-scroller': { fontFamily: 'Consolas, "SFMono-Regular", monospace', overflow: 'auto', lineHeight: '1.65' }, '.cm-content': { padding: '12px 0' }, '.cm-gutters': { backgroundColor: '#f7faf8', color: '#7a8d82', borderRight: '1px solid #e8eee9' }, '.cm-activeLine': { backgroundColor: '#edf8f180' }, '&.cm-focused': { outline: 'none' }, '.cm-selectionBackground, &.cm-focused .cm-selectionBackground': { backgroundColor: '#b6e6cd !important' }, '.cm-line': { padding: '0 10px' }, '.cm-tooltip': { border: '1px solid #d9e7df', borderRadius: '6px' } }),
  ] }) })
})
watch(() => props.modelValue, value => { if (view && value !== view.state.doc.toString()) view.dispatch({ changes: { from: 0, to: view.state.doc.length, insert: value } }) })
watch(wrap, value => view?.dispatch({ effects: wrapping.reconfigure(value ? EditorView.lineWrapping : []) }))
onBeforeUnmount(() => view?.destroy())
function command(callback) { if (view) { callback(view); view.focus() } }
function applyDiagnostics(items = []) {
  if (!view) return
  const diagnostics = (Array.isArray(items) ? items : []).map(item => {
    const lineNumber = Math.min(Math.max(1, Number(item.line) || 1), view.state.doc.lines)
    const line = view.state.doc.line(lineNumber)
    return { from: line.from, to: line.to, severity: item.severity || 'error', message: item.message || '运行错误' }
  })
  view.dispatch(setDiagnostics(view.state, diagnostics))
  if (diagnostics.length) view.dispatch({ effects: EditorView.scrollIntoView(diagnostics[0].from, { y: 'center' }) })
}
defineExpose({ focus: () => view?.focus(), setDiagnostics: applyDiagnostics })
</script>
<template>
  <div class="ide-editor">
    <div class="ide-tools"><button class="icon-button" title="撤销 Ctrl+Z" aria-label="撤销" @click="command(undo)"><Icon name="Undo2" :size="15" /></button><button class="icon-button" title="重做 Ctrl+Shift+Z" aria-label="重做" @click="command(redo)"><Icon name="Redo2" :size="15" /></button><button class="small-button" @click="openSearch(true)">查找 / 替换</button><button class="small-button" @click="command(gotoLine)">跳转行</button><button class="small-button" @click="command(startCompletion)">代码补全</button><label class="inline-check"><input v-model="wrap" type="checkbox" />自动换行</label><span class="grow" /><span class="muted">Ctrl+F 查找 · Tab 缩进 · Ctrl+S 保存</span></div>
    <div v-if="searchOpen" class="ide-search" role="search" @keydown.esc.stop.prevent="closeSearch" @keydown.ctrl.f.stop.prevent="reopenSearch" @keydown.ctrl.h.stop.prevent="replaceOpen = true; applyQuery()">
      <div class="ide-search-row">
        <Icon name="Search" :size="15" class="ide-search-icon" />
        <input ref="searchInput" v-model="searchText" class="ide-search-input" placeholder="查找内容" aria-label="查找内容" @input="applyQuery" @keydown.enter.prevent="onSearchEnter" />
        <span class="ide-search-count" :class="{ invalid: matchText === '正则无效' }">{{ matchText }}</span>
        <button type="button" class="ide-search-flag" :class="{ active: caseSensitive }" title="区分大小写" aria-label="区分大小写" @click="toggleOption('caseSensitive')">Aa</button>
        <button type="button" class="ide-search-flag" :class="{ active: wholeWord }" title="全字匹配" aria-label="全字匹配" @click="toggleOption('wholeWord')">ab</button>
        <button type="button" class="ide-search-flag" :class="{ active: useRegex }" title="正则表达式" aria-label="正则表达式" @click="toggleOption('useRegex')">.*</button>
        <button type="button" class="icon-button" title="上一个 (Shift+Enter)" aria-label="查找上一个" @click="searchPrev"><Icon name="ArrowUp" :size="16" /></button>
        <button type="button" class="icon-button" title="下一个 (Enter)" aria-label="查找下一个" @click="searchNext"><Icon name="ArrowDown" :size="16" /></button>
        <button type="button" class="ide-search-text" :class="{ active: replaceOpen }" @click="replaceOpen = !replaceOpen; replaceOpen && applyQuery()">替换</button>
        <button type="button" class="icon-button" title="关闭 (Esc)" aria-label="关闭查找" @click="closeSearch"><Icon name="X" :size="15" /></button>
      </div>
      <div v-if="replaceOpen" class="ide-search-row replace"><input v-model="replaceText" class="ide-search-input" placeholder="替换为…" aria-label="替换为" @input="applyQuery" @keydown.enter.prevent="replaceOne" /><button type="button" class="small-button" @click="replaceOne">替换</button><button type="button" class="small-button" @click="replaceAllMatches">全部替换</button><span class="ide-search-hint">Enter 替换当前 · 支持正则捕获组 $1</span></div>
    </div>
    <div ref="host" class="codemirror-host" />
    <footer class="editor-status"><span>{{ filename?.split('.').pop()?.toUpperCase() || 'TEXT' }} · UTF-8 · 4 空格</span><span>行:列 {{ position }} · {{ modelValue.split('\n').length }} 行</span></footer>
  </div>
</template>
