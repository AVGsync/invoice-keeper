import { reactive } from 'vue'

// Небольшой UI-стор. composerFocused поднимается, когда фокус в композере
// «Спросить»: App.vue тогда прячет плавающую панель вкладок, чтобы она не висела
// над экранной клавиатурой.
interface UiState {
  composerFocused: boolean
}

export const ui = reactive<UiState>({
  composerFocused: false,
})
