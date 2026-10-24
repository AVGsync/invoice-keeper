// apply theme / motion / font before the first screen is drawn, and again on every change
export default defineNuxtPlugin(() => {
  const { prefs } = usePrefs()
  watch(prefs, (value) => {
    applyPrefs(value)
    savePrefs(value)
  }, { deep: true, immediate: true })
})
