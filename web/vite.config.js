import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { writeFileSync } from 'node:fs'

const keepDistGitkeep = {
  name: 'keep-dist-gitkeep',
  closeBundle() {
    try { writeFileSync('dist/.gitkeep', '') } catch { /* dist 未生成时忽略 */ }
  },
}

export default defineConfig({
  plugins: [vue(), keepDistGitkeep],
  server: { proxy: Object.fromEntries(['/api', '/open', '/mcp'].map(path => [path, process.env.CABBAGE_DEV_API || 'http://127.0.0.1:5700'])) },
  build: { sourcemap: false, target: 'es2022', rollupOptions: { output: { manualChunks(id) { if (id.includes('@lezer/javascript') || id.includes('@codemirror/lang-javascript')) return 'language-javascript'; if (id.includes('@lezer/python') || id.includes('@codemirror/lang-python')) return 'language-python' } } } },
})
