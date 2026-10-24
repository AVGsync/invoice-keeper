// screens live under /user/... and /admin/..., the url tells which one we're on.
// later this should come from api.auth.me()
export function useRole() {
  const route = useRoute()
  const role = computed(() => (route.path.startsWith('/admin') ? 'admin' : 'user'))
  const isAdmin = computed(() => role.value === 'admin')
  return { role, isAdmin }
}
