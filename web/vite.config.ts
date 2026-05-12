import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

const apiTargets = ['/events', '/openapi.json', '/docs', '/elevators', '/floors', '/simulation']

export default defineConfig({
  plugins: [react()],
  build: {
    // Go の embed.FS が見える場所に出力する。
    outDir: '../internal/interface/http/server/webdist',
    // emptyOutDir: false にして webdist/.gitignore を残す。
    // 古いハッシュ付き artifact は累積するが、いずれも gitignore 対象なので実害なし。
    emptyOutDir: false,
  },
  server: {
    port: 5173,
    proxy: Object.fromEntries(
      apiTargets.map((p) => [p, { target: 'http://localhost:8080', changeOrigin: false, ws: false }])
    ),
  },
})
