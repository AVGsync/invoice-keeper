import { createApp } from 'vue'
import '@fontsource-variable/geist/wght.css'
import './styles/base.css'
import App from './App.vue'
import { router } from './router'
import { initSession, boot } from './stores/session'

const app = createApp(App)
app.use(router)

initSession(router, () => {
})

app.mount('#app')

void boot()
