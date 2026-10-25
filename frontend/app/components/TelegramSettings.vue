<!-- telegram of the logged in user: link the account to get reminders from the bot in a private chat.
     an admin also sees the team chat the reminders go to -->
<script setup lang="ts">
import type { TelegramLinkResponse, TelegramTeamChatResponse } from '~/types/api'

const api = useApi()
const { isAdmin } = useRole()

// undefined while loading
const link = ref<TelegramLinkResponse>()
const team = ref<TelegramTeamChatResponse | null>()
const loadError = ref('')

async function load() {
  loadError.value = ''
  try {
    link.value = await api.telegram.me()
  }
  catch (e) {
    loadError.value = apiError(e)
  }
  if (isAdmin.value) team.value = await api.telegram.teamChat().catch(() => null)
}

onMounted(load)

// one-time link to the bot. the user opens it, confirms in telegram and comes back here
const deepLink = ref('')
const linkUntil = ref('')
const busy = ref(false)
const error = ref('')

async function getLink() {
  busy.value = true
  error.value = ''
  try {
    const res = await api.telegram.linkCode()
    deepLink.value = res.deep_link ?? ''
    const until = new Date(Date.now() + (res.expires_in ?? 0) * 1000)
    linkUntil.value = res.expires_in ? until.toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' }) : ''
  }
  catch (e) {
    error.value = apiError(e)
  }
  finally {
    busy.value = false
  }
}

function openLink() {
  window.open(deepLink.value, '_blank', 'noopener')
}

// after confirming in telegram the user comes back to this tab: look again
async function check() {
  try {
    link.value = await api.telegram.me()
    if (link.value.linked) deepLink.value = ''
  }
  catch {
    // the next try or a reload will show it
  }
}

function onFocus() {
  if (deepLink.value) check()
}
onMounted(() => window.addEventListener('focus', onFocus))
onBeforeUnmount(() => window.removeEventListener('focus', onFocus))

// unlinking stops the private reminders, so the first click only asks
const confirmUnlink = ref(false)
async function unlink() {
  if (!confirmUnlink.value) {
    confirmUnlink.value = true
    return
  }
  confirmUnlink.value = false
  busy.value = true
  error.value = ''
  try {
    await api.telegram.unlink()
  }
  catch (e) {
    // 404: it was not linked anyway
    if ((e as { statusCode?: number }).statusCode !== 404) error.value = apiError(e)
  }
  finally {
    busy.value = false
  }
  await check()
}

const linkedText = computed(() => [
  link.value?.tg_username ? `@${link.value.tg_username}` : '',
  link.value?.linked_at ? `привязан ${formatDate(link.value.linked_at)}` : '',
].filter(Boolean).join(' · '))

// admin: test message to the team chat
const testState = ref<'idle' | 'busy' | 'sent'>('idle')
const testError = ref('')
async function sendTest() {
  testState.value = 'busy'
  testError.value = ''
  try {
    await api.telegram.teamChatTest()
    testState.value = 'sent'
    setTimeout(() => (testState.value = 'idle'), 2500)
  }
  catch (e) {
    testError.value = apiError(e)
    testState.value = 'idle'
  }
}
</script>

<template>
  <div class="telegram">
    <UiState v-if="loadError" kind="error" @retry="load">Не удалось загрузить: {{ loadError }}</UiState>

    <!-- grey rows of the same size until the answer arrives -->
    <template v-else-if="!link">
      <div v-for="n in (isAdmin ? 3 : 2)" :key="n" class="row">
        <span class="stack"><UiSkeleton width="170px" height="14px" /><UiSkeleton width="230px" height="10px" /></span>
        <UiSkeleton width="120px" height="36px" />
      </div>
    </template>

    <template v-else>
      <p class="group kicker">Личные напоминания</p>

      <div class="row">
        <div>
          <p>Аккаунт Telegram</p>
          <p v-if="link.linked" class="hint">{{ linkedText || 'Привязан' }}</p>
          <p v-else class="hint">Бот будет писать в личку о ваших платежах</p>
        </div>
        <Transition name="swap" mode="out-in">
          <span v-if="link.linked" key="linked" class="state ok"><span class="dot" />привязан</span>
          <UiButton v-else-if="!deepLink" key="link" size="md" variant="primary" :disabled="busy" @click="getLink">Привязать</UiButton>
          <span v-else key="waiting" class="state off waiting"><span class="dot" />ждём подтверждения</span>
        </Transition>
      </div>

      <!-- the link is ready: open it in telegram and confirm there -->
      <Transition name="unfold">
        <div v-if="!link.linked && deepLink" class="unfold">
          <div class="clip">
            <div class="row">
              <div class="link-text">
                <p>Откройте ссылку и нажмите кнопку подтверждения в боте</p>
                <p class="hint mono link">{{ deepLink }}</p>
                <p v-if="linkUntil" class="hint">Ссылка действует до {{ linkUntil }}</p>
              </div>
              <span class="actions">
                <UiButton size="md" @click="check">Проверить</UiButton>
                <UiButton size="md" variant="primary" @click="openLink">Открыть в Telegram</UiButton>
              </span>
            </div>
          </div>
        </div>
      </Transition>

      <div v-if="link.linked && link.bot_blocked" class="row">
        <div>
          <p class="amber">Бот заблокирован</p>
          <p class="hint">Напоминания не дойдут, пока вы не разблокируете бота в Telegram</p>
        </div>
      </div>

      <div v-if="link.linked" class="row">
        <div>
          <p>Отвязать Telegram</p>
          <p class="hint">Личные напоминания перестанут приходить</p>
        </div>
        <button type="button" class="unlink" :class="{ confirm: confirmUnlink }" :disabled="busy" @click="unlink" @blur="confirmUnlink = false">
          {{ confirmUnlink ? 'Точно отвязать?' : 'Отвязать' }}
        </button>
      </div>

      <p v-if="error" class="row error-text">{{ error }}</p>

      <template v-if="isAdmin">
        <p class="group kicker">Командный чат</p>

        <div class="row">
          <div>
            <p>{{ team?.bound ? team.title || 'Командный чат' : 'Не привязан' }}</p>
            <p v-if="team?.bound" class="hint">
              {{ team.bound_at ? `Привязан ${formatDate(team.bound_at)}` : 'Сюда уходят напоминания команды' }}
            </p>
            <p v-else-if="team === null" class="hint">Не удалось узнать состояние чата</p>
            <p v-else class="hint">Добавьте бота в группу команды и отправьте там /bind от своего имени</p>
          </div>
          <span v-if="team?.bound" class="state" :class="team.is_active === false ? 'off' : 'ok'">
            <span class="dot" />{{ team.is_active === false ? 'неактивен' : 'активен' }}
          </span>
        </div>

        <div v-if="team?.last_error" class="row">
          <div>
            <p class="red">Последняя ошибка доставки</p>
            <p class="hint">{{ team.last_error }}</p>
          </div>
        </div>

        <div v-if="team?.bound" class="row">
          <p v-if="testError" class="error-text">{{ testError }}</p>
          <p v-else-if="testState === 'sent'" class="hint ok">Отправлено, проверьте чат</p>
          <p v-else class="hint">Проверить, что бот может писать в чат</p>
          <UiButton size="md" :disabled="testState === 'busy'" @click="sendTest">Тестовое сообщение</UiButton>
        </div>
      </template>
    </template>
  </div>
</template>

<style scoped>
/* same rows as the other settings sections: hairlines, no box */
.telegram {
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

/* a small label above a group of rows */
.group {
  padding: 18px 12px 8px;
  border-bottom: 1px solid var(--line);
}

.stack {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.hint {
  margin-top: 2px;
  font-size: 12px;
  color: var(--ink-3);
}

.link-text {
  min-width: 0;
}

/* the link itself, to copy by hand if the button doesn't open telegram */
.link {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  user-select: all;
}

.actions {
  display: flex;
  flex-shrink: 0;
  gap: 8px;
}

/* linked / waiting with a dot, the same as on «О системе» */
.state {
  display: inline-flex;
  flex-shrink: 0;
  align-items: center;
  gap: 8px;
  font-size: 13px;
}

.state .dot {
  position: relative;
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: currentColor;
}

/* waiting for the bot: a ring keeps spreading out of the dot */
.waiting .dot::after {
  content: '';
  position: absolute;
  inset: 0;
  border-radius: 50%;
  background: currentColor;
  animation: waiting 1.6s var(--ease-out) infinite;
}

@keyframes waiting {
  from {
    opacity: 0.7;
  }

  to {
    opacity: 0;
    transform: scale(3);
  }
}

/* just linked: the green dot pops in */
.ok .dot {
  animation: linked 0.5s var(--ease-spring);
}

@keyframes linked {
  from {
    transform: scale(0);
  }
}

.ok {
  color: var(--green);
}

.off {
  color: var(--ink-3);
}

.amber {
  color: var(--amber);
}

.red {
  color: var(--red);
}

.unlink {
  flex-shrink: 0;
  padding: 0;
  background: none;
  border: 0;
  font-size: 13px;
  color: var(--ink-2);
}

.unlink:hover {
  color: var(--ink);
}

.unlink.confirm {
  color: var(--red);
  font-weight: 500;
}

/* phones */
@media (max-width: 1240px) {
  .row {
    flex-wrap: wrap;
    gap: 10px 14px;
    padding: 12px 16px 12px 12px;
  }
}
</style>
