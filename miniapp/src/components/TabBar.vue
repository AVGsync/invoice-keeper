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
  { to: '/', name: 'today', label: 'Сегодня', icon: 'M4 5h16v15H4zM4 9h16M9 3v4M15 3v4M11 13h2v2h-2z' },
  { to: '/payments', name: 'payments', label: 'Платежи', icon: 'M3 6h18v12H3zM3 10h18M7 15h4' },
  { to: '/calendar', name: 'calendar', label: 'Календарь', icon: 'M4 4h16v16H4zM4 9h16M9 9v11M15 9v11M4 14.5h16' },
  { to: '/ask', name: 'ask', label: 'Спросить', icon: 'M4 5h16v11H9l-5 4z' },
  { to: '/profile', name: 'profile', label: 'Профиль', icon: 'M12 12a4 4 0 1 0 0-8 4 4 0 0 0 0 8zM4 21c1-4 4.5-6 8-6s7 2 8 6' },
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
      <svg class="icon" viewBox="0 0 24 24" aria-hidden="true" focusable="false">
        <path :d="t.icon" />
      </svg>
      <span class="label">{{ t.label }}</span>
    </RouterLink>
  </nav>
</template>

<style scoped>
.tab {
  position: relative;
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
  padding: 10px 2px 8px;
  color: var(--fog);
  font-size: var(--fs-micro);
  text-transform: uppercase;
  letter-spacing: 0.02em;
  user-select: none;
  transition: color var(--dur) var(--ease);
}
@media (max-width: 374px) {
  .tab {
    text-transform: none;
    letter-spacing: 0.01em;
  }
}
.label {
  max-width: 100%;
  overflow: hidden;
  white-space: nowrap;
}
.tab.active {
  color: var(--bone);
}
.icon {
  width: 22px;
  height: 22px;
  fill: none;
  stroke: currentColor;
  stroke-width: 1.25;
  stroke-linecap: square;
}
.tab::before {
  content: '';
  position: absolute;
  top: -1px;
  left: 20%;
  right: 20%;
  height: 1px;
  background: var(--bone);
  transform: scaleX(0);
  transition: transform var(--dur) var(--ease);
}
.tab.active::before {
  transform: scaleX(1);
}
</style>
