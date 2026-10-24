import { reactive } from 'vue'

// Небольшой UI-стор.
interface UiState {
  composerFocused: boolean
}

export const ui = reactive<UiState>({
  composerFocused: false,
})
