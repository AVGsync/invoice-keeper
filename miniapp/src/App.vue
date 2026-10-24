<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { session } from './stores/session'
import { occurrences } from './stores/occurrences'
import { getWebApp, supports } from './telegram/webapp'
import { useBackButton } from './composables/useBackButton'
import TabBar from './components/TabBar.vue'
import InlineBack from './components/InlineBack.vue'
import BootScreen from './screens/BootScreen.vue'
import LinkScreen from './screens/LinkScreen.vue'
import FatalScreen from './screens/FatalScreen.vue'

const route = useRoute()
const { showInlineBack, goBack } = useBackButton()

// activated (8.0+): обновляем данные, если они старше 60 с.
onMounted(() => {
  const tg = getWebApp()
  if (tg && supports('8.0')) {
    tg.onEvent('activated', () => {
      if (session.phase === 'ready') void occurrences.load()
    })
  }
})

// Панель живёт всю сессию и уезжает вниз на экранах без вкладок — без пересоздания.
const showTabBar = computed(() => session.phase === 'ready')
const tabBarAway = computed(() => !route.meta.tab)
const isFatal = computed(() =>
  ['no_telegram', 'expired', 'invalid', 'disabled', 'rate_limited', 'offline'].includes(
    session.phase,
  ),
)
</script>

<template>
  <div class="container">
    <BootScreen v-if="session.phase === 'booting'" />
    <LinkScreen v-else-if="session.phase === 'link'" />
    <FatalScreen v-else-if="isFatal" />
    <template v-else>
      <InlineBack v-if="showInlineBack && !route.meta.tab" @back="goBack" />
      <RouterView v-slot="{ Component, route: r }">
        <Transition name="page">
          <component :is="Component" :key="r.path" />
        </Transition>
      </RouterView>
    </template>
  </div>
  <TabBar
    v-if="showTabBar"
    :class="{ 'docked-away': tabBarAway }"
    :aria-hidden="tabBarAway || undefined"
    :inert="tabBarAway || undefined"
  />
</template>
