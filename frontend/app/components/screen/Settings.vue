<script setup lang="ts">
const { prefs } = usePrefs()
const demo = useDemo()
const api = useApi()

const themes = [
  { value: 'dark', label: 'Тёмная' },
  { value: 'light', label: 'Светлая' },
]
// segmented works with plain strings, prefs want the exact union
const theme = computed({
  get: () => prefs.value.theme,
  set: value => (prefs.value.theme = value as 'dark' | 'light'),
})

const resetDone = ref(false)
function resetDemo() {
  demo.reset()
  resetDone.value = true
}

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

const serverText = { checking: 'проверяем…', ok: 'на связи', down: 'нет связи' }
const assistantText = { checking: 'проверяем…', ok: 'работает', down: 'недоступен', off: 'выключен' }
</script>

<template>
  <div>
    <AppPageHeader title="Настройки" subtitle="Хранятся в этом браузере" />

    <div class="groups">
      <section class="card group">
        <h2 class="title display">Оформление</h2>
        <div class="row">
          <span>Тема</span>
          <UiSegmented v-model="theme" :options="themes" />
        </div>
      </section>

      <section class="card group">
        <h2 class="title display">Доступность</h2>
        <div class="row">
          <div>
            <p>Меньше движения</p>
            <p class="hint">Без анимаций и плавных переходов</p>
          </div>
          <UiSwitch v-model="prefs.lessMotion" aria-label="Меньше движения" />
        </div>
        <div class="row">
          <div>
            <p>Шрифт для дислексии</p>
            <p class="hint">Andika: буквы проще различать</p>
          </div>
          <UiSwitch v-model="prefs.readableFont" aria-label="Шрифт для дислексии" />
        </div>
      </section>

      <section class="card group">
        <h2 class="title display">Демо-режим</h2>
        <div class="row">
          <div>
            <p>Демо-режим</p>
            <p class="hint">Кнопки «скоро» работают на демо-данных, только в этом браузере</p>
          </div>
          <UiSwitch v-model="prefs.demoMode" aria-label="Демо-режим" />
        </div>
        <div class="row">
          <div>
            <p>Демо-данные</p>
            <p class="hint">{{ resetDone ? 'Сброшены к исходным' : 'Вернуть платежи как были' }}</p>
          </div>
          <UiButton size="md" @click="resetDemo">Сбросить</UiButton>
        </div>
      </section>

      <section class="card group">
        <h2 class="title display">О системе</h2>
        <UiLeaderRow>
          <template #label>Сервер</template>
          <span :class="server">{{ serverText[server] }}</span>
        </UiLeaderRow>
        <UiLeaderRow>
          <template #label>Ассистент</template>
          <span :class="assistant">{{ assistantText[assistant] }}</span>
        </UiLeaderRow>
        <UiLeaderRow>
          <template #label>Часовой пояс браузера</template>
          {{ timezone }}
        </UiLeaderRow>
      </section>
    </div>
  </div>
</template>

<style scoped>
.groups {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(360px, 1fr));
  gap: 16px;
  max-width: 1000px;
}

.group {
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding: 18px 20px 20px;
}

.title {
  font-size: 20px;
}

.row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
  font-size: 13px;
}

.hint {
  margin-top: 2px;
  font-size: 12px;
  color: var(--ink-3);
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
</style>
