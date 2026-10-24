<!-- email reminders of the logged in user, kept on the server (/notifications/settings) -->
<script setup lang="ts">
import type { UserNotificationSettings } from '~/types/api'

const api = useApi()

// undefined while loading, null when they were never set up (the server answers 404)
const saved = ref<UserNotificationSettings | null>()
const loadError = ref('')

const form = reactive({
  email: '',
  enabled: true,
  days: '3',
  time: '09:00',
  timezone: Intl.DateTimeFormat().resolvedOptions().timeZone || 'Europe/Moscow',
})

function fill(settings: UserNotificationSettings) {
  form.email = settings.email ?? ''
  form.enabled = settings.email_enabled ?? true
  form.days = String(settings.days_before_due ?? 3)
  form.time = settings.send_time ?? '09:00'
  form.timezone = settings.timezone || form.timezone
}

async function load() {
  loadError.value = ''
  try {
    saved.value = await api.notifications.get()
    fill(saved.value)
  }
  catch (e) {
    if ((e as { statusCode?: number }).statusCode === 404) saved.value = null
    else loadError.value = apiError(e)
  }
}

onMounted(load)

const busy = ref(false)
const error = ref('')
const done = ref(false)

async function save() {
  const days = Number(form.days)
  if (!/^\S+@\S+\.\S+$/.test(form.email.trim())) error.value = 'Введите почту'
  else if (!Number.isInteger(days) || days < 0 || days > 365) error.value = 'Дней до срока: от 0 до 365'
  else if (!/^([01]\d|2[0-3]):[0-5]\d$/.test(form.time)) error.value = 'Время в формате ЧЧ:ММ'
  else {
    error.value = ''
    busy.value = true
    const body = { email: form.email.trim(), email_enabled: form.enabled, days_before_due: days, send_time: form.time, timezone: form.timezone }
    try {
      // first time: create, after that: replace
      if (saved.value) await api.notifications.replace(body)
      else await api.notifications.create(body)
      await load()
      done.value = true
      setTimeout(() => (done.value = false), 2500)
    }
    catch (e) {
      error.value = apiError(e)
    }
    finally {
      busy.value = false
    }
  }
}
</script>

<template>
  <div class="notify">
    <UiState v-if="loadError" kind="error" @retry="load">Не удалось загрузить: {{ loadError }}</UiState>
    <p v-else-if="saved === undefined" class="row hint">Загружаем…</p>

    <!-- one setting per row, like the other sections: what it is on the left, the field on the right -->
    <template v-else>
      <div class="row">
        <div>
          <p>Присылать письма</p>
          <p class="hint">О платежах, за которые вы отвечаете</p>
        </div>
        <UiSwitch v-model="form.enabled" aria-label="Присылать письма" />
      </div>

      <div class="row">
        <div>
          <p>Почта</p>
          <p v-if="saved?.email" class="hint" :class="saved.email_verified ? 'ok' : 'off'">
            {{ saved.email_verified ? 'Подтверждена' : 'Не подтверждена' }}
          </p>
          <p v-else class="hint">Смена почты снимает подтверждение</p>
        </div>
        <span class="field wide"><UiInput v-model="form.email" type="email" placeholder="name@company.ru" autocomplete="email" aria-label="Почта" /></span>
      </div>

      <div class="row">
        <div>
          <p>За сколько дней до срока</p>
          <p class="hint">От 0 до 365</p>
        </div>
        <span class="field"><UiInput v-model="form.days" mono inputmode="numeric" maxlength="3" aria-label="За сколько дней до срока" /></span>
      </div>

      <div class="row">
        <div>
          <p>Во сколько</p>
          <p class="hint">Часовой пояс: {{ form.timezone }}</p>
        </div>
        <span class="field"><UiInput v-model="form.time" mono placeholder="09:00" maxlength="5" aria-label="Во сколько" /></span>
      </div>

      <div class="row">
        <p v-if="error" class="error-text">{{ error }}</p>
        <p v-else-if="done" class="hint ok">Сохранено</p>
        <span v-else />
        <UiButton size="md" variant="primary" :disabled="busy" @click="save">
          {{ saved ? 'Сохранить' : 'Включить напоминания' }}
        </UiButton>
      </div>
    </template>
  </div>
</template>

<style scoped>
/* same rows as the other settings sections: hairlines, no box */
.notify {
  display: flex;
  flex-direction: column;
  border-top: 1px solid var(--line);
}

.row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
  min-height: 56px;
  padding: 10px 44px 10px 12px;
  border-bottom: 1px solid var(--line);
  font-size: 14px;
}

.field {
  flex-shrink: 0;
  width: 120px;
}

.field.wide {
  width: 320px;
}

.hint {
  margin-top: 2px;
  font-size: 12px;
  color: var(--ink-3);
}

.ok {
  color: var(--green);
}

.off {
  color: var(--amber);
}
</style>
