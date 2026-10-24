import type { LoginRequest, User } from '~/types/api'

export function useAuth() {
  const api = useApi()
  // undefined = not checked yet, null = not logged in
  const user = useState<User | null | undefined>('auth-user', () => undefined)
  const resetSession = useSessionReset()

  async function load() {
    try {
      user.value = await api.auth.me()
    }
    catch {
      // access cookie lives 1 hour, refresh cookie 7 days, so try once to get a new pair
      try {
        await api.auth.refresh()
        user.value = await api.auth.me()
      }
      catch {
        // the session is over: whatever the last person had loaded goes too
        resetSession()
        user.value = null
      }
    }
  }

  async function login(body: LoginRequest) {
    await api.auth.login(body)
    // the person logging in starts clean, even if the last one never pressed «Выйти»
    resetSession()
    user.value = await api.auth.me()
  }

  // cleared even if the request fails: the data must not stay for the next person
  async function logout() {
    try {
      await api.auth.logout()
    }
    finally {
      resetSession()
      user.value = null
    }
  }

  const home = computed(() => (user.value?.role === 'admin' ? '/admin/overview' : '/user/overview'))

  return { user, load, login, logout, home }
}
