import { apiGroups } from '../web/src/apidocs.js'
import { writeFile } from 'node:fs/promises'

const output = new URL('../internal/panel/mcp_apidocs.json', import.meta.url)
const payload = { generatedAt: new Date().toISOString(), groups: apiGroups }
await writeFile(output, JSON.stringify(payload, null, 2) + '\n')
const endpoints = apiGroups.reduce((total, group) => total + group.endpoints.length, 0)
console.log(`Wrote ${apiGroups.length} groups / ${endpoints} endpoints to ${output.pathname}`)
