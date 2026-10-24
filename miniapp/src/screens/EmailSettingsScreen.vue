<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import {
  getNotificationSettings,
  saveNotificationSettings,
  deleteNotificationSettings,
  sendEmailCode,
  verifyEmail,
} from '../api/endpoints'
import { humanError } from '../lib/humanError'
import { haptic, confirm as tgConfirm, alert as tgAlert } from '../telegram/webapp'
import { useMainButton } from '../composables/useMainButton'
import Segmented from '../components/Segmented.vue'
import Section from '../components/Section.vue'
import Cell from '../components/Cell.vue'
import Skeleton from '../components/Skeleton.vue'
import type { NotificationSettings } from '../api/types'

const loaded = ref(false)
const saved = ref<NotificationSettings | null>(null)
const email = ref('')
const enabled = ref(true)
const days = ref(3)
const time = ref('09:00')
const timezone = ref('Europe/Moscow')
const busy = ref(false)
const formError = ref('')
const code = ref('')
const codeSent = ref(false)

const ZONES = [
  'Europe/Kaliningrad', 'Europe/Moscow', 'Europe/Samara', 'Asia/Yekaterinburg', 'Asia/Omsk',
  'Asia/Novosibirsk', 'Asia/Krasnoyarsk', 'Asia/Irkutsk', 'Asia/Yakutsk', 'Asia/Vladivostok',
  'Asia/Magadan', 'Asia/Kamchatka', 'Europe/Kyiv', 'Asia/Almaty', 'UTC',
]
const zones = computed(() => (ZONES.includes(timezone.value) ? ZONES : [timezone.value, ...ZONES]))
const DAYS = [0, 1, 2, 3, 5, 7, 14]

function dayLabel(n: number): string {
  if (n === 0) return 'в день срока'
  if (n === 1) return 'за 1 день'
  return `за ${n} дн.`
}

onMounted(async () => {
  try {
    const s = await getNotificationSettings()
    if (s) apply(s)
    else timezone.value = Intl.DateTimeFormat().resolvedOptions().timeZone || 'Europe/Moscow'
  } catch (e) {
    formError.value = humanError(e)
  } finally {
    loaded.value = true
  }
})

function apply(s: NotificationSettings): void {
  saved.value = s
  email.value = s.email
  enabled.value = s.email_enabled
  days.value = s.days_before_due
  time.value = s.send_time
  timezone.value = s.timezone
}

const EMAIL_RE = /^[^@\s]+@[^@\s]+\.[^@\s]+$/
const emailValid = computed(() => EMAIL_RE.test(email.value.trim()))
const dirty = computed(
  () =>
    !saved.value ||
    email.value.trim() !== saved.value.email ||
    enabled.value !== saved.value.email_enabled ||
    days.value !== saved.value.days_before_due ||
    time.value !== saved.value.send_time ||
    timezone.value !== saved.value.timezone,
)
const valid = computed(() => emailValid.value && /^\d{2}:\d{2}$/.test(time.value) && dirty.value)

async function run(action: () => Promise<void>): Promise<boolean> {
  if (busy.value) return false
  busy.value = true
  formError.value = ''
  try {
    await action()
    haptic.success()
    return true
  } catch (e) {
    haptic.error()
    formError.value = humanError(e)
    return false
  } finally {
    busy.value = false
  }
}

async function save(): Promise<void> {
  if (!valid.value) return
  const ok = await run(async () => {
    await saveNotificationSettings(!!saved.value, {
      email: email.value.trim(),
      email_enabled: enabled.value,
      days_before_due: days.value,
      send_time: time.value,
      timezone: timezone.value,
    })
    const fresh = await getNotificationSettings()
    if (fresh) apply(fresh)
  })
  if (ok && saved.value && !saved.value.email_verified) codeSent.value = false
}

async function requestCode(): Promise<void> {
  const ok = await run(async () => {
    await sendEmailCode()
  })
  if (ok) {
    codeSent.value = true
    await tgAlert('Код отправлен на ' + saved.value?.email + '. Проверьте также «Спам».')
  }
}

async function confirmCode(): Promise<void> {
  const ok = await run(async () => {
    await verifyEmail(code.value.trim())
    const fresh = await getNotificationSettings()
    if (fresh) apply(fresh)
  })
  if (ok) code.value = ''
}

async function remove(): Promise<void> {
  if (!saved.value || !(await tgConfirm('Удалить настройки почты? Письма приходить перестанут.'))) return
  const ok = await run(async () => {
    await deleteNotificationSettings()
  })
  if (ok) {
    saved.value = null
    email.value = ''
    codeSent.value = false
  }
}

useMainButton({ text: 'Сохранить', enabled: valid, loading: busy, onClick: save })
</script>

<template>
  <div class="mail">
    <h1 class="title">Почта</h1>
    <Skeleton v-if="!loaded" :rows="4" />
    <template v-else>
      <p class="hint intro">
        Письмо со сводкой приходит один раз в день, когда есть платежи, срок которых близко или уже прошёл. Только по
        вашим платежам. Сообщения в Telegram-чате команды это не заменяет.
      </p>

      <label class="field">
        <span class="lbl">Адрес</span>
        <input v-model="email" type="email" inputmode="email" autocomplete="email" autocapitalize="off" />
        <span v-if="email && !emailValid" class="err">Введите адрес вида name@example.com</span>
      </label>

      <div class="field">
        <span class="lbl">Письма</span>
        <Segmented
          :model-value="enabled ? 'on' : 'off'"
          :options="[
            { value: 'on', label: 'Включены' },
            { value: 'off', label: 'Выключены' },
          ]"
          @update:model-value="(v) => (enabled = v === 'on')"
        />
      </div>

      <label class="field">
        <span class="lbl">Напоминать</span>
        <select v-model.number="days">
          <option v-for="d in DAYS" :key="d" :value="d">{{ dayLabel(d) }}</option>
        </select>
      </label>

      <label class="field">
        <span class="lbl">Во сколько</span>
        <input v-model="time" type="time" />
      </label>

      <label class="field">
        <span class="lbl">Часовой пояс</span>
        <select v-model="timezone">
          <option v-for="z in zones" :key="z" :value="z">{{ z }}</option>
        </select>
      </label>

      <Section v-if="saved" title="Подтверждение адреса">
        <Cell label="Статус">
          {{ saved.email_verified ? 'Адрес подтверждён' : 'Не подтверждён: письма не отправляются' }}
        </Cell>
        <div v-if="!saved.email_verified" class="verify">
          <button class="btn btn-secondary btn-block" type="button" :disabled="busy || dirty" @click="requestCode">
            {{ codeSent ? 'Отправить код ещё раз' : 'Отправить код на почту' }}
          </button>
          <p v-if="dirty" class="hint">Сначала сохраните изменения.</p>
          <template v-if="codeSent">
            <input
              v-model="code"
              type="text"
              inputmode="numeric"
              autocomplete="one-time-code"
              maxlength="6"
              placeholder="Код из письма"
              aria-label="Код из письма"
            />
            <button class="btn btn-block" type="button" :disabled="busy || code.trim().length !== 6" @click="confirmCode">
              Подтвердить
            </button>
          </template>
        </div>
      </Section>

      <p v-if="formError" class="form-error">{{ formError }}</p>

      <button v-if="saved" class="btn btn-destructive remove" type="button" :disabled="busy" @click="remove">
        Удалить настройки
      </button>
    </template>
  </div>
</template>

<style scoped>
.intro {
  margin: 0 0 var(--sp-6);
}
.verify {
  display: flex;
  flex-direction: column;
  gap: var(--sp-3);
  padding: var(--sp-4) 0;
}
.remove {
  margin-top: var(--sp-6);
}
</style>
