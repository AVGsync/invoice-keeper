// runs a server search whenever its parameters change. it waits a moment after the last change
// (so typing doesn't fire a request per letter) and keeps only the newest answer.
// params() returns null when there is nothing to search for: then the result is cleared
// the new rows come in through a view transition: rows with a view-transition-name slide to their place
export function useServerSearch<P, R>(params: () => P | null, run: (params: P) => Promise<R>) {
  const { withTransition } = useViewTransition()
  const result = shallowRef<R>()
  const searching = ref(false)
  const error = ref('')

  let timer: ReturnType<typeof setTimeout> | undefined
  let latest = 0

  async function search(current: P) {
    const mine = ++latest
    searching.value = true
    error.value = ''
    try {
      const answer = await run(current)
      if (mine === latest) withTransition(() => { result.value = answer })
    }
    catch (e) {
      if (mine === latest) error.value = apiError(e)
    }
    finally {
      if (mine === latest) searching.value = false
    }
  }

  watch(() => JSON.stringify(params()), () => {
    clearTimeout(timer)
    const current = params()
    if (!current) {
      latest++
      // back to the full list, the same slide
      if (result.value !== undefined) withTransition(() => { result.value = undefined })
      searching.value = false
      error.value = ''
      return
    }
    // the first answer is still awaited: don't show the old one meanwhile
    searching.value = true
    timer = setTimeout(() => search(current), 300)
  }, { immediate: true })

  onBeforeUnmount(() => clearTimeout(timer))

  function retry() {
    const current = params()
    if (current) search(current)
  }

  return { result, searching, error, retry }
}
