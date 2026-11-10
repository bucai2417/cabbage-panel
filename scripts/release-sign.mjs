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
  const key = process.env.RELEASE_PRIVATE_KEY
  if (!key) throw new Error('RELEASE_PRIVATE_KEY is required')
  const publicKey = createPublicKey(key).export({ type: 'spki', format: 'der' }).subarray(-32).toString('base64')
  if (publicKey !== process.env.CABBAGE_UPDATE_PUBLIC_KEY) throw new Error('Signing key does not match build public key')
  const assets = []
  for (const name of (await readdir(directory)).sort()) {
    const match = /^cabbage-(linux|windows|darwin)-(amd64|arm64)(?:\.exe)?$/.exec(name)
    if (!match) continue
    const bytes = await readFile(path.join(directory, name))
    assets.push({ os: match[1], arch: match[2], name, sha256: createHash('sha256').update(bytes).digest('hex') })
  }
  if (!assets.length) throw new Error('No release binaries found')
  const manifest = Buffer.from(JSON.stringify({ version, assets }, null, 2) + '\n')
  await writeFile(path.join(directory, 'manifest.json'), manifest)
  await writeFile(path.join(directory, 'manifest.sig'), sign(null, manifest, key).toString('base64') + '\n')
  await writeFile(path.join(directory, 'SHA256SUMS'), assets.map(asset => asset.sha256 + '  ' + asset.name).join('\n') + '\n')
  console.log('Signed release ' + version + ' with ' + assets.length + ' assets')
}
