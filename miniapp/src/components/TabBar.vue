<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { assistant } from '../stores/assistant'
import { haptic } from '../telegram/webapp'

const route = useRoute()

interface Tab {
  to: string
  name: string
  label: string
  icon: string
}

const allTabs: Tab[] = [
  { to: '/', name: 'today', label: 'Сегодня', icon: '📅' },
  { to: '/payments', name: 'payments', label: 'Платежи', icon: '💳' },
  { to: '/calendar', name: 'calendar', label: 'Календарь', icon: '🗓' },
  { to: '/ask', name: 'ask', label: 'Спросить', icon: '💬' },
  { to: '/profile', name: 'profile', label: 'Профиль', icon: '👤' },
]

const tabs = computed(() =>
  allTabs.filter((t) => t.name !== 'ask' || assistant.status.enabled),
)

function isActive(name: string): boolean {
  return route.name === name
}

function onTap(name: string): void {
  if (!isActive(name)) haptic.selection()
}
</script>

<template>
  <nav class="tabbar">
    <RouterLink
      v-for="t in tabs"
      :key="t.name"
      :to="t.to"
      class="tab"
      :class="{ active: isActive(t.name) }"
      @click="onTap(t.name)"
    >
      <span class="icon">{{ t.icon }}</span>
      <span class="label">{{ t.label }}</span>
    </RouterLink>
  </nav>
</template>

<style scoped>
.tab {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 2px;
  padding: 8px 0;
  color: var(--c-hint);
  font-size: 11px;
  user-select: none;
}
.tab.active {
  color: var(--c-primary);
}
.icon {
  font-size: 20px;
}
</style>
