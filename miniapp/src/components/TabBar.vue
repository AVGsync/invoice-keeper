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

const activeIndex = computed(() => tabs.value.findIndex((t) => t.name === route.name))

function isActive(name: string): boolean {
  return route.name === name
}

function onTap(name: string): void {
  if (!isActive(name)) haptic.selection()
}
</script>

<template>
  <nav class="tabbar">
    <span
      class="indicator"
      :class="{ off: activeIndex < 0 }"
      :style="{ '--i': Math.max(activeIndex, 0), '--n': tabs.length }"
      aria-hidden="true"
    />
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
/* glass pill that glides to the active tab (spatial consistency) */
.indicator {
  position: absolute;
  top: var(--tb-pad);
  bottom: var(--tb-pad);
  left: var(--tb-pad);
  width: calc((100% - var(--tb-pad) * 2) / var(--n));
  border-radius: 22px;
  background: var(--pill);
  box-shadow: inset 0 0 0 1px var(--pill-ring);
  transform: translateX(calc(var(--i) * 100%));
  transition:
    transform 0.36s var(--ease-drawer),
    opacity 0.2s var(--ease-out);
  pointer-events: none;
}
.indicator.off {
  opacity: 0;
}
.tab {
  position: relative;
  z-index: 1;
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 3px;
  padding: 6px 2px;
  border-radius: 22px;
  color: var(--muted);
  font-size: var(--fs-micro);
  font-weight: 500;
  letter-spacing: 0.01em;
  user-select: none;
  transition: color 0.25s var(--ease-out);
}
@media (max-width: 374px) {
  .tab {
    font-size: 10px;
    letter-spacing: -0.01em;
    padding: 6px 0;
  }
}
.label {
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.tab.active {
  color: var(--bone-text);
}
.tab.active .icon {
  stroke: var(--accent-ink);
}
.icon {
  width: 22px;
  height: 22px;
  fill: none;
  stroke: currentColor;
  stroke-width: 1.5;
  stroke-linecap: round;
  stroke-linejoin: round;
  transition: transform 0.14s var(--ease-out);
}
/* press feedback */
.tab:active .icon {
  transform: scale(0.88);
}
.tab.active .icon {
  animation: pop 0.32s var(--ease-out);
}
@keyframes pop {
  from {
    transform: scale(0.86);
  }
}
</style>
