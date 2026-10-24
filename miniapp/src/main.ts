import { createApp } from 'vue'
import './styles/base.css'
import App from './App.vue'
import { router } from './router'
import { initSession, boot } from './stores/session'

const app = createApp(App)
app.use(router)

initSession(router, () => {
  // onReady: ничего дополнительного — App реагирует на session.phase.
})

app.mount('#app')

void boot()
