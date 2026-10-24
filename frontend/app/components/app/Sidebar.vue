<script setup lang="ts">
const { role, isAdmin } = useRole()

const mainNav = computed(() => [
  { label: 'Обзор', to: `/${role.value}/overview` },
  { label: 'Платежи', to: `/${role.value}/payments` },
  { label: 'Календарь', to: `/${role.value}/calendar` },
  { label: 'Регулярные', to: `/${role.value}/recurring` },
  // admins have it in their own section below
  ...(isAdmin.value ? [] : [{ label: 'Настройки', to: '/user/settings' }]),
])

const adminNav = [
  { label: 'Пользователи', to: '/admin/users' },
  { label: 'Персоны', to: '/admin/personas' },
  { label: 'Журнал', to: '/admin/log' },
  { label: 'Настройки', to: '/admin/settings' },
]

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

const { prefs } = usePrefs()
const demo = useDemo()
const isLight = computed(() => prefs.value.theme === 'light')
function toggleTheme() {
  prefs.value.theme = isLight.value ? 'dark' : 'light'
}
</script>

<template>
  <aside class="sidebar">
    <AppLogo />
    <div class="rule" />

    <nav class="nav">
      <template v-for="item in mainNav" :key="item.label">
        <NuxtLink :to="item.to" class="item" active-class="active">{{ item.label }}</NuxtLink>
      </template>

      <template v-if="isAdmin">
        <p class="kicker section">Администратор</p>
        <template v-for="item in adminNav" :key="item.label">
          <NuxtLink :to="item.to" class="item" active-class="active">{{ item.label }}</NuxtLink>
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
      <button type="button" class="logout" @click="signOut">Выйти</button>
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
  border-right: 1px solid var(--line);
}

.sidebar > .logo {
  padding-left: 12px;
}

.rule {
  margin: 16px 12px;
  border-top: 1px solid var(--line-strong);
}

.nav {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

/* picking an item fades its background in and grows the blue bar,
   the item you leave does the same backwards */
.item {
  position: relative;
  height: 32px;
  padding: 0 12px;
  border-radius: 6px;
  color: var(--ink-2);
  line-height: 32px;
  transition: background-color 0.2s ease, color 0.2s ease, font-weight 0.2s ease;
}

.item:hover:not(.active) {
  color: var(--ink);
}

.item.active {
  background: var(--muted-bg);
  color: var(--ink);
  font-weight: 500;
}

/* blue bar on the left, hidden (squashed) until the item is active */
.item::before {
  content: '';
  position: absolute;
  left: 0;
  top: 8px;
  width: 3px;
  height: 16px;
  border-radius: 2px;
  background: var(--accent);
  opacity: 0;
  transform: scaleY(0);
  transition: transform 0.2s ease, opacity 0.2s ease;
}

.item.active::before {
  opacity: 1;
  transform: scaleY(1);
}

.section {
  margin: 21px 12px 12px;
}

.demo {
  align-self: flex-start;
  margin: auto 12px 12px;
  padding: 2px 8px;
  border: 1px dashed var(--amber);
  border-radius: 4px;
  font-size: 11px;
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
  font-size: 11px;
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
  border-radius: 6px;
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
  font-size: 11px;
  color: var(--ink-3);
}

.me-loading {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.logout {
  margin-left: auto;
  padding: 0;
  background: none;
  border: 0;
  font-size: 12px;
  color: var(--red);
}
</style>
