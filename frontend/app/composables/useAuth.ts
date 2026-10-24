import type { LoginRequest, User } from '~/types/api'

export function useAuth() {
  const api = useApi()
  // undefined = not checked yet, null = not logged in
  const user = useState<User | null | undefined>('auth-user', () => undefined)

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
        user.value = null
      }
    }
  }

  async function login(body: LoginRequest) {
    await api.auth.login(body)
    user.value = await api.auth.me()
  }

  async function logout() {
    await api.auth.logout()
    user.value = null
  }

  const home = computed(() => (user.value?.role === 'admin' ? '/admin/overview' : '/user/overview'))

  return { user, load, login, logout, home }
}
