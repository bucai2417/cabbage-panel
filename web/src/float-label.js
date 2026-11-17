// 把输入框内的 placeholder 提示词变成可上浮的浮动标签。
//
// 原生 placeholder 由浏览器绘制在输入框内部，无法移出到边框线上。这里读取
// placeholder 文本、插入一个绝对定位的 .float-label 承担显示，并把原生
// placeholder 换成空格——保留 :placeholder-shown 伪类（CSS 用它判断空值）。
//
// 不重排 DOM：Vue 用 v-model 管理输入框，移动节点会破坏它的 patch。改为测量
// 输入框在父容器内的位置，通过 CSS 变量把浮标定位到「框内居中」或「上边框线」。

const SKIP_TYPES = new Set(['checkbox', 'radio', 'file', 'range', 'color', 'hidden'])
// 搜索框、工具栏等「图标 + 输入框」结构不适合浮动标签：浮标会和图标/布局冲突。
// 只处理表单字段（label.field 内的输入框）。
const SKIP_SELECTOR = '.global-search, .search-input, .filter-bar'

function measure(input, label) {
  // offsetTop/offsetHeight 相对最近的定位祖先，父容器已设为 position: relative
  label.style.setProperty('--float-rest', `${input.offsetTop + input.offsetHeight / 2}px`)
  label.style.setProperty('--float-lifted', `${input.offsetTop}px`)
}

function enhance(input) {
  if (!(input instanceof HTMLInputElement) || SKIP_TYPES.has(input.type)) return
  if (input.dataset.floatLabel === 'on') return
  if (input.closest(SKIP_SELECTOR)) return
  const text = input.getAttribute('placeholder')
  if (!text || !text.trim()) return

  const label = input.parentElement
  if (!label) return

  // 浮标插入后，label 元素的可访问名称会变成「标题 + 浮标文本」，破坏
  // getByLabel / 屏幕阅读器的匹配。用 label 标题给输入框补 aria-label，
  // 它的优先级高于 label 文本关联。
  const heading = label.querySelector(':scope > span:not(.float-label)')
  const accessibleName = (heading?.textContent || '').replace(/\*/g, '').trim()
  if (accessibleName && !input.getAttribute('aria-label')) {
    input.setAttribute('aria-label', accessibleName)
  }

  input.dataset.floatLabel = 'on'
  // 原生 placeholder 换成空格：不显示文字，但 :placeholder-shown 仍然有效。
  input.placeholder = ' '
  label.classList.add('float-wrap')

  const bubble = document.createElement('span')
  bubble.className = 'float-label'
  bubble.setAttribute('aria-hidden', 'true')
  bubble.textContent = text.trim()
  input.insertAdjacentElement('afterend', bubble)

  measure(input, label)
  // 容器尺寸或字体变化时重新测量（窗口缩放、双列断点切换、字体加载完成等）。
  if (typeof ResizeObserver !== 'undefined') {
    const observer = new ResizeObserver(() => measure(input, label))
    observer.observe(input)
  }
}

function scan(root) {
  if (!(root instanceof Element) && !(root instanceof Document)) return
  root.querySelectorAll('input[placeholder]').forEach(enhance)
  if (root instanceof HTMLInputElement && root.hasAttribute('placeholder')) enhance(root)
}

export function installFloatLabels() {
  if (typeof document === 'undefined') return

  const start = () => {
    scan(document)
    // 弹窗、动态表单在挂载后才出现，用观察器增量处理。
    const observer = new MutationObserver(records => {
      for (const record of records) {
        for (const node of record.addedNodes) {
          if (node.nodeType === Node.ELEMENT_NODE) scan(node)
        }
      }
    })
    observer.observe(document.body, { childList: true, subtree: true })
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', start, { once: true })
  } else {
    start()
  }
}
