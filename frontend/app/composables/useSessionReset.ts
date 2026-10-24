// everything the panel keeps in memory (payments, users, the chat) belongs to the person logged in.
// on logout, or when someone else logs in on the same tab, all of it goes back to empty,
// so the next person never sees the previous one's data
let generation = 0

// changes on every reset: a request that started before it drops its answer instead of showing it
export const sessionGeneration = () => generation

export function useSessionReset() {
  // taken here, in setup: the reset itself runs later from a click
  const nuxtApp = useNuxtApp()
  return () => nuxtApp.runWithContext(() => {
    generation++
    stopAssistant()
    // the browser's own settings (theme, less motion, font) are not about the person, they stay
    clearNuxtState(key => key !== 'prefs', { reset: true })
  })
}
