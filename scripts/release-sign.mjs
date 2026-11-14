import { generateKeyPairSync, createPublicKey, sign, createHash } from 'node:crypto'
import { readFile, writeFile, readdir } from 'node:fs/promises'
import path from 'node:path'

if (process.argv[2] === 'keygen') {
  const { privateKey, publicKey } = generateKeyPairSync('ed25519')
  await writeFile('release-private.key', privateKey.export({ type: 'pkcs8', format: 'pem' }), { mode: 0o600, flag: 'wx' })
  const rawKey = publicKey.export({ type: 'spki', format: 'der' }).subarray(-32).toString('base64')
  console.log('Private key written to release-private.key. Keep it offline or in GitHub Secrets.')
  console.log('CABBAGE_UPDATE_PUBLIC_KEY=' + rawKey)
} else {
  const directory = process.argv[2] || 'dist'
  const version = (await readFile('VERSION', 'utf8')).trim()
  const assets = []
  for (const name of (await readdir(directory)).sort()) {
    const match = /^cabbage-(linux|windows|darwin)-(amd64|arm64)(?:\.exe)?$/.exec(name)
    if (!match) continue
    const bytes = await readFile(path.join(directory, name))
    assets.push({ os: match[1], arch: match[2], name, sha256: createHash('sha256').update(bytes).digest('hex') })
  }
  if (!assets.length) throw new Error('No release binaries found')

  // 清单与校验和始终生成：应用无公钥时会据此校验 SHA-256。
  const manifest = Buffer.from(JSON.stringify({ version, assets }, null, 2) + '\n')
  await writeFile(path.join(directory, 'manifest.json'), manifest)
  await writeFile(path.join(directory, 'SHA256SUMS'), assets.map(asset => asset.sha256 + '  ' + asset.name).join('\n') + '\n')

  const key = process.env.RELEASE_PRIVATE_KEY
  const publicKeyVar = (process.env.CABBAGE_UPDATE_PUBLIC_KEY || '').trim()
  if (!key) {
    if (publicKeyVar) {
      console.warn('CABBAGE_UPDATE_PUBLIC_KEY is set but RELEASE_PRIVATE_KEY is missing; publishing UNSIGNED manifest. Updates will be verified by SHA-256 only.')
    } else {
      console.log('No signing key configured; publishing unsigned manifest. Updates will be verified by SHA-256 only.')
    }
    console.log('Generated manifest.json and SHA256SUMS (unsigned) for ' + assets.length + ' assets')
  } else {
    const publicKey = createPublicKey(key).export({ type: 'spki', format: 'der' }).subarray(-32).toString('base64')
    if (!publicKeyVar) throw new Error('RELEASE_PRIVATE_KEY is set but CABBAGE_UPDATE_PUBLIC_KEY is missing')
    if (publicKey !== publicKeyVar) throw new Error('Signing key does not match build public key')
    await writeFile(path.join(directory, 'manifest.sig'), sign(null, manifest, key).toString('base64') + '\n')
    console.log('Signed release ' + version + ' with ' + assets.length + ' assets')
  }
}
