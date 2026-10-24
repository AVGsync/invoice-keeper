import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname, resolve } from 'node:path'
import type { Plugin } from 'vite'
import { signInitData } from './initdata'

const here = dirname(fileURLToPath(import.meta.url))

// Dev-плагин: внедряет мок Telegram и свежеподписанный initData.
export function tgDev(env: Record<string, string>): Plugin {
  return {
    name: 'tg-dev',
    apply: 'serve',
    transformIndexHtml(html) {
      const botToken = env.DEV_TG_BOT_TOKEN || ''
      const userId = Number(env.DEV_TG_USER_ID || '100001')
      const firstName = env.DEV_TG_FIRST_NAME || 'Dev'
      const username = env.DEV_TG_USERNAME || 'devuser'
      const startParam = env.DEV_TG_START_PARAM || undefined
      const colorScheme = env.DEV_TG_COLOR_SCHEME === 'dark' ? 'dark' : 'light'

      const initData = botToken
        ? signInitData({
            botToken,
            user: {
              id: userId,
              first_name: firstName,
              username,
              language_code: 'ru',
              allows_write_to_pm: true,
            },
            startParam,
            authDate: Math.floor(Date.now() / 1000),
          })
        : ''

      const mockSource = readFileSync(resolve(here, 'telegram-mock.js'), 'utf-8')
      const config = JSON.stringify({ initData, startParam, colorScheme })

      return {
        html,
        tags: [
          {
            tag: 'script',
            injectTo: 'head',
            children: `window.__TG_MOCK_CONFIG__=${config}`,
          },
          {
            tag: 'script',
            injectTo: 'head',
            children: mockSource,
          },
        ],
      }
    },
  }
}
