import { defineConfig, loadEnv } from 'vite'
import vue from '@vitejs/plugin-vue'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { tgDev } from './dev/vite-plugin-tg-dev'

const pkg = JSON.parse(
  readFileSync(fileURLToPath(new URL('./package.json', import.meta.url)), 'utf-8'),
)

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '')
  return {
    base: '/tg/',
    plugins: [vue(), tgDev(env)],
    server: {
      port: 5173,
      proxy: {
        '/api': {
          target: env.DEV_API_TARGET || 'http://localhost:8080',
          changeOrigin: true,
        },
      },
    },
    preview: {
      port: 4173,
    },
    build: {
      target: 'es2020',
      assetsDir: 'assets',
      sourcemap: false,
    },
    define: {
      __APP_VERSION__: JSON.stringify(pkg.version),
    },
  }
})
