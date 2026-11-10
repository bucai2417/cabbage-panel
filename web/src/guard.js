const guards = new Set()

export function registerLeaveGuard(guard) {
  guards.add(guard)
  return () => guards.delete(guard)
}

export async function checkLeaveGuards() {
  for (const guard of [...guards]) {
    try {
      if (!(await guard())) return false
    } catch {
      return false
    }
  }
  return true
}
