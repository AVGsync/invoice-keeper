// login cookies only exist in the browser, so the check runs there
export default defineNuxtRouteMiddleware(async (to) => {
  if (import.meta.server) return

  const { user, load, home } = useAuth()
  if (user.value === undefined) await load()

  if (to.path === '/login') {
    if (user.value) return navigateTo(home.value)
    return
  }

  if (!user.value) return navigateTo('/login')

  // a user can't open admin screens, an admin gets the admin version of a user screen
  if (user.value.role !== 'admin' && to.path.startsWith('/admin')) return navigateTo(home.value)
  if (user.value.role === 'admin' && to.path.startsWith('/user')) return navigateTo(to.path.replace('/user', '/admin'))
})
