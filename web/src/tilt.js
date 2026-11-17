// 统计卡片 3D 倾斜：指针在卡片内移动时，卡片按指针相对中心的位置轻微旋转，
// 离开后归位。只在支持悬停的精细指针设备启用；开启「减少动效」时完全不挂监听。
//
// 倾斜角度很小（最大 5°），只用来表达"卡片被注视"的层次感，不做夸张翻转。

const MAX_TILT = 5
const TARGET = '.stat-card'

export function installCardTilt() {
  if (typeof window === 'undefined' || typeof document === 'undefined') return

  const precise = window.matchMedia('(hover: hover) and (pointer: fine)')
  const reduced = window.matchMedia('(prefers-reduced-motion: reduce)')
  if (!precise.matches || reduced.matches) return

  const cardOf = event => (event.target instanceof Element ? event.target.closest(TARGET) : null)

  const reset = card => {
    if (!card) return
    card.style.removeProperty('--tilt-x')
    card.style.removeProperty('--tilt-y')
  }

  document.addEventListener('pointermove', event => {
    if (reduced.matches) return
    const card = cardOf(event)
    if (!card) return
    const rect = card.getBoundingClientRect()
    if (!rect.width || !rect.height) return
    // -0.5 ~ 0.5：指针在卡片内的相对位置
    const dx = (event.clientX - rect.left) / rect.width - 0.5
    const dy = (event.clientY - rect.top) / rect.height - 0.5
    // 指针偏右时卡片右侧后仰，偏下时下侧后仰，接近"视线跟随"
    card.style.setProperty('--tilt-x', `${(-dy * MAX_TILT * 2).toFixed(2)}deg`)
    card.style.setProperty('--tilt-y', `${(dx * MAX_TILT * 2).toFixed(2)}deg`)
  }, { passive: true })

  // 指针移出卡片时归位；relatedTarget 仍在卡片内则忽略。
  document.addEventListener('pointerout', event => {
    const card = cardOf(event)
    if (!card) return
    const next = event.relatedTarget
    if (next instanceof Node && card.contains(next)) return
    reset(card)
  }, { passive: true })

  // 系统偏好在使用中切换为减少动效时，清掉已有倾斜。
  reduced.addEventListener?.('change', () => {
    document.querySelectorAll(TARGET).forEach(reset)
  })
}
