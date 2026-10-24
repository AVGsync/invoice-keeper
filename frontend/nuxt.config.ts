// https://nuxt.com/docs/api/configuration/nuxt-config
export default defineNuxtConfig({
  compatibilityDate: '2025-07-15',
  devtools: { enabled: true },
  modules: ['@nuxt/eslint'],
  css: ['~/assets/css/main.css'],
  app: {
    head: {
      htmlAttrs: { lang: 'ru' },
      title: 'Invoice Keeper',
      link: [
        // browsers cache favicons hard: bump ?v= whenever public/favicon.ico changes
        { rel: 'icon', type: 'image/x-icon', href: '/favicon.ico?v=2' },
        { rel: 'preconnect', href: 'https://fonts.googleapis.com' },
        { rel: 'preconnect', href: 'https://fonts.gstatic.com', crossorigin: '' },
        {
          rel: 'stylesheet',
          href: 'https://fonts.googleapis.com/css2?family=Geist+Mono:wght@400;500;600&family=Geologica:slnt,wght@-12..0,400..600&family=Unbounded:wght@400;500;600&display=swap',
        },
      ],
    },
  },
  nitro: {
    // dev only, without it the login cookies don't work on localhost. in prod nginx does this
    devProxy: {
      '/api/v1': { target: 'https://77.239.105.84/api/v1', changeOrigin: true },
    },
  },
})
