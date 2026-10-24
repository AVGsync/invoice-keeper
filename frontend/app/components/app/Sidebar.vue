<script setup lang="ts">
import type { IconName } from '~/components/ui/Icon.vue'

const { role, isAdmin } = useRole()

interface NavItem { label: string, to: string, icon: IconName }

const mainNav = computed<NavItem[]>(() => [
  { label: 'Обзор', to: `/${role.value}/overview`, icon: 'overview' },
  { label: 'Платежи', to: `/${role.value}/payments`, icon: 'payments' },
  { label: 'Календарь', to: `/${role.value}/calendar`, icon: 'calendar' },
  { label: 'Регулярные', to: `/${role.value}/recurring`, icon: 'recurring' },
  // admins have it in their own section below
  ...(isAdmin.value ? [] : [{ label: 'Настройки', to: '/user/settings', icon: 'settings' } as NavItem]),
])

const adminNav: NavItem[] = [
  { label: 'Пользователи', to: '/admin/users', icon: 'users' },
  { label: 'Журнал', to: '/admin/log', icon: 'log' },
  { label: 'Настройки', to: '/admin/settings', icon: 'settings' },
]

// one highlight slides from the old page's item to the new one, across both sections
const root = ref<HTMLElement>()
const marker = ref<{ top: number, height: number }>()
const route = useRoute()

function placeMarker() {
  const active = root.value?.querySelector<HTMLElement>('.item.active')
  marker.value = active ? { top: active.offsetTop, height: active.offsetHeight } : undefined
}

watch(() => route.path, () => nextTick(placeMarker))
onMounted(placeMarker)

const { user: me, logout } = useAuth()

async function signOut() {
  // even if the request fails the cookies are useless now, go to login anyway
  await logout().catch(() => {})
  await navigateTo('/login')
}

const user = computed(() => ({
  initials: initials(me.value?.full_name),
  name: me.value?.full_name ?? '',
  // admin always comes from .env, users show their persona
  role: me.value?.role === 'admin'
    ? 'admin · .env'
    : ['user', me.value?.persona?.name].filter(Boolean).join(' · '),
}))

// backend has no bot status endpoint yet, so it's always online for now
const botOnline = ref(true)

const { prefs, switchTheme } = usePrefs()
const demo = useDemo()
const isLight = computed(() => prefs.value.theme === 'light')
function toggleTheme(e: MouseEvent) {
  const box = (e.currentTarget as HTMLElement).getBoundingClientRect()
  switchTheme(isLight.value ? 'dark' : 'light', { x: box.left + box.width / 2, y: box.top + box.height / 2 })
}
</script>

<template>
  <aside ref="root" class="sidebar" :class="{ placed: marker }">
    <AppLogo />
    <div class="rule" />

    <span
      v-if="marker"
      class="marker"
      :style="{ transform: `translateY(${marker.top}px)`, height: `${marker.height}px` }"
      aria-hidden="true"
    />

    <nav class="nav">
      <template v-for="item in mainNav" :key="item.label">
        <NuxtLink :to="item.to" class="item" active-class="active">
          <UiIcon :name="item.icon" class="item-icon" />{{ item.label }}
        </NuxtLink>
      </template>

      <template v-if="isAdmin">
        <p class="kicker section">Администратор</p>
        <template v-for="item in adminNav" :key="item.label">
          <NuxtLink :to="item.to" class="item" active-class="active">
            <UiIcon :name="item.icon" class="item-icon" />{{ item.label }}
          </NuxtLink>
        </template>
      </template>
    </nav>

    <NuxtLink v-if="demo.enabled.value" :to="`/${role}/settings`" class="demo mono" title="Действия меняют только демо-данные в этом браузере">
      демо-режим
    </NuxtLink>

    <div class="status">
      <p class="bot mono">
        <span class="bot-dot" :class="{ offline: !botOnline }" />TG-бот · {{ botOnline ? 'онлайн' : 'оффлайн' }}
      </p>
      <button
        type="button"
        class="theme"
        :aria-label="isLight ? 'Тёмная тема' : 'Светлая тема'"
        :title="isLight ? 'Тёмная тема' : 'Светлая тема'"
        @click="toggleTheme"
      >
        <!-- moon in the light theme, sun in the dark one: shows where the click takes you -->
        <svg v-if="isLight" viewBox="0 0 16 16" width="14" height="14" aria-hidden="true">
          <path d="M13.5 9.8A5.5 5.5 0 0 1 6.2 2.5a5.5 5.5 0 1 0 7.3 7.3Z" fill="currentColor" />
        </svg>
        <svg v-else viewBox="0 0 16 16" width="14" height="14" aria-hidden="true">
          <circle cx="8" cy="8" r="3" fill="currentColor" />
          <path d="M8 1v2M8 13v2M1 8h2M13 8h2M3 3l1.4 1.4M11.6 11.6 13 13M3 13l1.4-1.4M11.6 4.4 13 3" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" />
        </svg>
      </button>
    </div>

    <div class="me">
      <!-- the server doesn't know who's logged in, so the profile is drawn only in the browser.
           otherwise vue keeps the skeleton's styles on the avatar (hydration mismatch) -->
      <ClientOnly>
        <template v-if="me">
          <UiAvatar :initials="user.initials" :size="30" />
          <div>
            <p class="me-name">{{ user.name }}</p>
            <p class="me-role mono">{{ user.role }}</p>
          </div>
        </template>
        <template #fallback>
          <UiSkeleton width="30px" height="30px" round />
          <span class="me-loading"><UiSkeleton width="100px" /><UiSkeleton width="70px" height="10px" /></span>
        </template>
      </ClientOnly>
      <button type="button" class="logout" aria-label="Выйти" title="Выйти" @click="signOut">
        <UiIcon name="logout" class="logout-icon" />
      </button>
    </div>
  </aside>
</template>

<style scoped>
.sidebar {
  position: sticky;
  top: 0;
  display: flex;
  flex-direction: column;
  width: 233px;
  height: 100vh;
  padding: 24px 12px 0;
  background: var(--side);
  border-right: 1px solid var(--line);
}

.sidebar > .logo {
  padding-left: 12px;
}

.rule {
  margin: 16px 12px;
  border-top: 1px solid var(--line);
}

.nav {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

/* the active item sits on a soft blue fill (.marker), no side bar */
.item {
  position: relative;
  display: flex;
  align-items: center;
  gap: 10px;
  height: 34px;
  padding: 0 12px;
  border-radius: 9999px;
  color: var(--ink-2);
  font-weight: 500;
  transition: background-color 0.2s ease, color 0.2s ease;
}

.item:hover:not(.active) {
  background: var(--subtle);
  color: var(--ink);
}

.item.active {
  color: var(--ink);
}

/* until the marker is placed (first paint) the item paints its own fill */
.sidebar:not(.placed) .item.active {
  background: var(--accent-bg);
}

.item-icon {
  color: var(--ink-3);
  transition: color 0.2s ease, transform 0.45s var(--ease-spring);
}

.item:hover .item-icon {
  color: var(--ink);
  transform: scale(1.12);
}

.item.active .item-icon {
  color: var(--accent-ink);
}

/* the sliding fill behind the active item */
.marker {
  position: absolute;
  top: 0;
  left: 12px;
  right: 12px;
  border-radius: 9999px;
  background: var(--accent-bg);
  transition: transform 0.5s var(--ease-out), height 0.5s var(--ease-out);
}

.section {
  margin: 21px 12px 12px;
}

.demo {
  align-self: flex-start;
  margin: auto 12px 12px;
  padding: 2px 9px;
  border-radius: 9999px;
  background: var(--amber-bg);
  font-size: 12px;
  font-weight: 500;
  color: var(--amber);
}

.status {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin: auto 4px 12px 12px;
}

.demo + .status {
  margin-top: 0;
}

.bot {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
  color: var(--ink-2);
}

.theme {
  display: grid;
  place-items: center;
  width: 28px;
  height: 28px;
  padding: 0;
  background: none;
  border: 1px solid var(--line);
  border-radius: 9999px;
  color: var(--ink-2);
  transition: color 0.2s ease, border-color 0.2s ease;
}

.theme:hover {
  color: var(--ink);
  border-color: var(--line-strong);
}

.bot-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--green);
}

.bot-dot.offline {
  background: var(--red);
}

.me {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 16px 12px 24px;
  border-top: 1px solid var(--line);
}

.me-name {
  font-size: 13px;
  font-weight: 500;
}

.me-role {
  font-size: 12px;
  color: var(--ink-3);
}

.me-loading {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.logout {
  display: grid;
  place-items: center;
  flex-shrink: 0;
  width: 28px;
  height: 28px;
  margin-left: auto;
  padding: 0;
  background: none;
  border: 1px solid transparent;
  border-radius: 9999px;
  font-size: 12px;
  color: var(--red);
}
.logout-icon {
  transition: transform 0.35s var(--ease-out);
}

.logout:hover {
  background: var(--red-bg);
}

.logout:hover .logout-icon {
  transform: translateX(2px);
}
</style>
