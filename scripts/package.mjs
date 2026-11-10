import { readFile, writeFile, readdir } from 'node:fs/promises'
import { createHash } from 'node:crypto'
import { spawnSync } from 'node:child_process'
import path from 'node:path'

const version = (await readFile('VERSION', 'utf8')).trim()
const directory = path.resolve('dist')
const binaries = (await readdir(directory)).filter(name => /^cabbage-(linux|windows|darwin)-(amd64|arm64)(?:\.exe)?$/.test(name)).sort()
const checksums = []
for (const name of binaries) {
  checksums.push(createHash('sha256').update(await readFile(path.join(directory, name))).digest('hex') + '  ' + name)
}
await writeFile(path.join(directory, 'SHA256SUMS'), checksums.join('\n') + '\n')
const script = [
  'import pathlib,zipfile,sys',
  'root=pathlib.Path.cwd()',
  'version=(root/"VERSION").read_text(encoding="utf-8").strip()',
  'binaries=sys.argv[1:]',
  'for name in binaries:',
  '    target=root/"dist"/(name.removesuffix(".exe")+"-v"+version+".zip")',
  '    with zipfile.ZipFile(target,"w",zipfile.ZIP_DEFLATED,compresslevel=9) as archive:',
  '        archive.write(root/"dist"/name,name)',
  '        for relative in ["README.md","AGENTS.md","CHANGELOG.md","SECURITY.md","THIRD_PARTY_NOTICES.md","VERSION","docs/API.md","docs/RELEASE.md","docs/TESTING.md","docs/WORKSPACE.md","docs/BUILD.md","docs/DEVELOPMENT.md","dist/SHA256SUMS"]:',
  '            archive.write(root/relative,"SHA256SUMS" if relative=="dist/SHA256SUMS" else relative)',
  '    print(target.name)',
].join('\n')
const result = spawnSync(process.platform === 'win32' ? 'python' : 'python3', ['-c', script, ...binaries], { stdio: 'inherit', windowsHide: true })
if (result.status !== 0) throw new Error('Package generation failed')
console.log('Packaged v' + version + ' for ' + binaries.length + ' targets; runtime data and secrets excluded.')
