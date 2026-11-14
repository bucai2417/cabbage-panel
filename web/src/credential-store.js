const STORAGE_KEY = 'cabbage-remembered-password'
const FALLBACK_KEY = 'cabbage-remembered-password-key'
const DB_NAME = 'cabbage-credentials'
const DB_STORE = 'keys'
const DB_KEY = 'remembered-password'
const encoder = new TextEncoder()
const decoder = new TextDecoder()

function encodeBase64(bytes) {
  let binary = ''
  for (let index = 0; index < bytes.length; index++) binary += String.fromCharCode(bytes[index])
  return btoa(binary)
}

function decodeBase64(value) {
  const binary = atob(value)
  const bytes = new Uint8Array(binary.length)
  for (let index = 0; index < binary.length; index++) bytes[index] = binary.charCodeAt(index)
  return bytes
}

function randomBytes(length) {
  return crypto.getRandomValues(new Uint8Array(length))
}

function xorBytes(bytes, key) {
  const output = new Uint8Array(bytes.length)
  for (let index = 0; index < bytes.length; index++) output[index] = bytes[index] ^ key[index % key.length]
  return output
}

function openDatabase() {
  return new Promise((resolve, reject) => {
    const request = indexedDB.open(DB_NAME, 1)
    request.onupgradeneeded = () => {
      const database = request.result
      if (!database.objectStoreNames.contains(DB_STORE)) database.createObjectStore(DB_STORE)
    }
    request.onsuccess = () => resolve(request.result)
    request.onerror = () => reject(request.error)
  })
}

function readStoredKey(database) {
  return new Promise((resolve, reject) => {
    const request = database.transaction(DB_STORE, 'readonly').objectStore(DB_STORE).get(DB_KEY)
    request.onsuccess = () => resolve(request.result || null)
    request.onerror = () => reject(request.error)
  })
}

function writeStoredKey(database, key) {
  return new Promise((resolve, reject) => {
    const transaction = database.transaction(DB_STORE, 'readwrite')
    transaction.objectStore(DB_STORE).put(key, DB_KEY)
    transaction.oncomplete = () => resolve(key)
    transaction.onerror = () => reject(transaction.error)
  })
}

async function cryptoKey(create) {
  const database = await openDatabase()
  try {
    let key = await readStoredKey(database)
    if (!key && create) {
      key = await crypto.subtle.generateKey({ name: 'AES-GCM', length: 256 }, false, ['encrypt', 'decrypt'])
      await writeStoredKey(database, key)
    }
    return key
  } finally {
    database.close()
  }
}

function fallbackKey() {
  let stored = ''
  try { stored = localStorage.getItem(FALLBACK_KEY) || '' } catch { /* 忽略存储失败 */ }
  if (stored) return decodeBase64(stored)
  const bytes = randomBytes(32)
  try { localStorage.setItem(FALLBACK_KEY, encodeBase64(bytes)) } catch { /* 忽略存储失败 */ }
  return bytes
}

export function clearRememberedPassword() {
  try { localStorage.removeItem(STORAGE_KEY) } catch { /* 忽略存储失败 */ }
}

export async function saveRememberedPassword(password) {
  if (!password) {
    clearRememberedPassword()
    return false
  }
  try {
    if (typeof crypto !== 'undefined' && crypto.subtle && typeof indexedDB !== 'undefined') {
      try {
        const key = await cryptoKey(true)
        const iv = randomBytes(12)
        const cipher = new Uint8Array(await crypto.subtle.encrypt({ name: 'AES-GCM', iv }, key, encoder.encode(password)))
        localStorage.setItem(STORAGE_KEY, JSON.stringify({ version: 2, iv: encodeBase64(iv), data: encodeBase64(cipher) }))
        return true
      } catch { /* 非安全上下文或 IndexedDB 不可用时降级 */ }
    }
    const data = xorBytes(encoder.encode(password), fallbackKey())
    localStorage.setItem(STORAGE_KEY, JSON.stringify({ version: 1, data: encodeBase64(data) }))
    return true
  } catch {
    return false
  }
}

export async function loadRememberedPassword() {
  let raw = ''
  try { raw = localStorage.getItem(STORAGE_KEY) || '' } catch { return '' }
  if (!raw) return ''
  try {
    const payload = JSON.parse(raw)
    if (payload.version === 2 && typeof crypto !== 'undefined' && crypto.subtle && typeof indexedDB !== 'undefined') {
      const key = await cryptoKey(false)
      if (!key) return ''
      const plain = await crypto.subtle.decrypt({ name: 'AES-GCM', iv: decodeBase64(payload.iv) }, key, decodeBase64(payload.data))
      return decoder.decode(plain)
    }
    if (payload.version === 1) {
      return decoder.decode(xorBytes(decodeBase64(payload.data), fallbackKey()))
    }
  } catch {
    clearRememberedPassword()
  }
  return ''
}
