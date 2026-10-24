<script setup lang="ts">
const { role, isAdmin } = useRole()

const mainNav = computed(() => [
  { label: 'Обзор', to: `/${role.value}/overview` },
  { label: 'Платежи', to: `/${role.value}/payments` },
  { label: 'Календарь', to: `/${role.value}/calendar` },
  { label: 'Регулярные', to: `/${role.value}/recurring` },
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

    <p class="bot mono">
      <span class="bot-dot" :class="{ offline: !botOnline }" />TG-бот · {{ botOnline ? 'онлайн' : 'оффлайн' }}
    </p>

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

.bot {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: auto 12px 20px;
  font-size: 11px;
  color: var(--ink-2);
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
