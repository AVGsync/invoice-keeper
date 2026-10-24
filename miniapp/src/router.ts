import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import { session, isAdmin } from './stores/session'
import { assistant } from './stores/assistant'

const routes: RouteRecordRaw[] = [
  { path: '/', name: 'today', component: () => import('./screens/TodayScreen.vue'), meta: { tab: true } },
  {
    path: '/payments',
    name: 'payments',
    component: () => import('./screens/PaymentsScreen.vue'),
    meta: { tab: true },
  },
  {
    path: '/occ/:id',
    name: 'occ',
    component: () => import('./screens/OccurrenceScreen.vue'),
    meta: { parent: '/' },
  },
  {
    path: '/calendar',
    name: 'calendar',
    component: () => import('./screens/CalendarScreen.vue'),
    meta: { tab: true },
  },
  { path: '/ask', name: 'ask', component: () => import('./screens/AskScreen.vue'), meta: { tab: true } },
  {
    path: '/profile',
    name: 'profile',
    component: () => import('./screens/ProfileScreen.vue'),
    meta: { tab: true },
  },
  {
    path: '/admin/recurring',
    name: 'recurring',
    component: () => import('./screens/admin/RecurringListScreen.vue'),
    meta: { admin: true, parent: '/profile' },
  },
  {
    path: '/admin/recurring/new',
    name: 'recurringNew',
    component: () => import('./screens/admin/RecurringCreateScreen.vue'),
    meta: { admin: true, parent: '/admin/recurring' },
  },
  {
    path: '/admin/users',
    name: 'users',
    component: () => import('./screens/admin/UsersScreen.vue'),
    meta: { admin: true, parent: '/profile' },
  },
  {
    path: '/admin/users/new',
    name: 'userNew',
    component: () => import('./screens/admin/UserCreateScreen.vue'),
    meta: { admin: true, parent: '/admin/users' },
  },
  { path: '/:pathMatch(.*)*', redirect: '/' },
]

export const router = createRouter({
  history: createWebHistory('/tg/'),
  routes,
})

router.beforeEach((to) => {
  if (to.meta.admin && session.phase === 'ready' && !isAdmin.value) return '/'
  if (to.name === 'ask' && session.phase === 'ready' && !assistant.status.enabled) return '/'
  return true
})
