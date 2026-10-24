<script setup lang="ts">
import type { IconName } from '~/components/ui/Icon.vue'

const { prefs } = usePrefs()
const api = useApi()
const route = useRoute()

// like settings on a phone: a list of sections, a click opens one, «Настройки» goes back.
// the open section lives in the url (?section=), so the browser back button works too
type Section = 'access' | 'mail' | 'telegram' | 'about'
const sections: { id: Section, title: string, icon: IconName }[] = [
  { id: 'access', title: 'Доступность', icon: 'access' },
  { id: 'mail', title: 'Напоминания на почту', icon: 'mail' },
  { id: 'telegram', title: 'Telegram', icon: 'send' },
  { id: 'about', title: 'О системе', icon: 'info' },
]
const open = computed(() => sections.find(item => item.id === route.query.section))

function show(id?: Section) {
  navigateTo({ query: id ? { section: id } : {} })
}

// short state next to each menu item
const summary = computed<Record<Section, string>>(() => ({
  access: [prefs.value.lessMotion && 'меньше движения', prefs.value.readableFont && 'шрифт'].filter(Boolean).join(', ') || 'выкл',
  mail: '',
  telegram: '',
  about: serverText[server.value],
}))

// quick health check of what the panel talks to
const server = ref<'checking' | 'ok' | 'down'>('checking')
const assistant = ref<'checking' | 'ok' | 'down' | 'off'>('checking')
const timezone = Intl.DateTimeFormat().resolvedOptions().timeZone

onMounted(async () => {
  api.ping().then(() => (server.value = 'ok'), () => (server.value = 'down'))
  try {
    const status = await api.assistant.status()
    if (status.enabled === false) assistant.value = 'off'
    else assistant.value = status.healthy ? 'ok' : 'down'
  }
  catch {
    assistant.value = 'down'
  }
})

const serverText = { checking: 'проверяем…', ok: 'онлайн', down: 'офлайн' }
const assistantText = { checking: 'проверяем…', ok: 'онлайн', down: 'офлайн', off: 'выключен' }
</script>

<template>
  <div>
    <AppPageHeader title="Настройки" />

    <div class="settings">
      <Transition name="drill" mode="out-in">
        <!-- the list of sections -->
        <nav v-if="!open" key="menu" class="list" aria-label="Разделы настроек">
          <button
            v-for="(item, i) in sections"
            :key="item.id"
            type="button"
            class="item rise"
            :style="{ '--i': i }"
            @click="show(item.id)"
          >
            <span class="badge"><UiIcon :name="item.icon" :size="17" /></span>
            <span class="name">{{ item.title }}</span>
            <span class="value">{{ summary[item.id] }}</span>
            <UiIcon name="right" :size="16" class="chevron" />
          </button>
        </nav>

        <!-- one section -->
        <section v-else :key="open.id" class="sub">
          <button type="button" class="back" @click="show()">
            <UiIcon name="left" :size="16" />Настройки
          </button>
          <h2 class="sub-title display">
            <span class="badge"><UiIcon :name="open.icon" :size="17" /></span>{{ open.title }}
          </h2>

          <div v-if="open.id === 'access'" class="list">
            <div class="row">
              <p>Меньше движения</p>
              <UiSwitch v-model="prefs.lessMotion" aria-label="Меньше движения" />
            </div>
            <div class="row">
              <p>Шрифт для дислексии</p>
              <UiSwitch v-model="prefs.readableFont" aria-label="Шрифт для дислексии" />
            </div>
          </div>

          <NotificationSettings v-else-if="open.id === 'mail'" />

          <TelegramSettings v-else-if="open.id === 'telegram'" />

          <div v-else class="list">
            <div class="row">
              <span>Сервер</span>
              <span class="state" :class="server"><span class="dot" />{{ serverText[server] }}</span>
            </div>
            <div class="row">
              <span>Ассистент</span>
              <span class="state" :class="assistant"><span class="dot" />{{ assistantText[assistant] }}</span>
            </div>
            <div class="row">
              <span>Часовой пояс браузера</span>
              <span class="mono">{{ timezone }}</span>
            </div>
          </div>
        </section>
      </Transition>
    </div>
  </div>
</template>

<style scoped>
/* the hairlines run to the right edge of the window (past the page's side padding),
   the row content keeps that padding */
.settings {
  margin-right: -32px;
}

/* rows with hairlines between them, no boxes.
   pulled up so its top line lies right on the header's line instead of making a second one */
.list {
  display: flex;
  flex-direction: column;
  margin-top: -19px;
  border-top: 1px solid var(--line);
}

.item,
.row {
  display: flex;
  align-items: center;
  gap: 14px;
  min-height: 56px;
  padding: 10px 44px 10px 12px;
  border-bottom: 1px solid var(--line);
}

.item {
  width: 100%;
  background: none;
  border-top: 0;
  border-right: 0;
  border-left: 0;
  font-size: 14px;
  font-weight: 500;
  text-align: left;
  transition: background-color 0.2s ease;
}

.item:hover {
  background: var(--subtle);
}

.row {
  justify-content: space-between;
  font-size: 14px;
}

.badge {
  display: inline-grid;
  place-items: center;
  flex-shrink: 0;
  width: 32px;
  height: 32px;
  border-radius: 9999px;
  background: var(--accent-bg);
  color: var(--accent-ink);
}

.name {
  flex: 1;
}

.value {
  font-size: 13px;
  font-weight: 400;
  color: var(--ink-3);
}

.chevron {
  color: var(--ink-3);
  transition: transform 0.35s var(--ease-out);
}

.item:hover .chevron {
  transform: translateX(3px);
  color: var(--ink);
}

.back {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  margin: 0 0 10px -6px;
  padding: 4px 10px 4px 6px;
  background: none;
  border: 0;
  border-radius: 9999px;
  font-size: 13px;
  font-weight: 500;
  color: var(--accent-ink);
  transition: background-color 0.2s ease;
}

.back:hover {
  background: var(--accent-bg);
}

.sub-title {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 16px;
  font-size: 20px;
  line-height: 28px;
}

.hint {
  margin-top: 2px;
  font-size: 12px;
  color: var(--ink-3);
}

/* online / offline with a dot, the same as the bot line in the sidebar */
.state {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
}

.state .dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: currentColor;
}

.ok {
  color: var(--green);
}

.down {
  color: var(--red);
}

.off,
.checking {
  color: var(--ink-3);
}

/* going into a section slides it in from the right, back slides the list in */
.drill-enter-active {
  transition: opacity 0.25s ease, transform 0.45s var(--ease-out);
}

.drill-leave-active {
  transition: opacity 0.12s ease;
}

.drill-enter-from {
  opacity: 0;
  transform: translateX(24px);
}

.drill-leave-to {
  opacity: 0;
}
</style>
