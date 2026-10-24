<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { session, isAdmin } from '../stores/session'
import { getTelegramLink } from '../api/endpoints'
import { useAsync } from '../composables/useAsync'
import { formatDateTime } from '../lib/dates'
import Section from '../components/Section.vue'
import Cell from '../components/Cell.vue'

const router = useRouter()
const me = computed(() => session.me)
const roleLabel = computed(() => (me.value?.role === 'admin' ? 'Администратор' : 'Сотрудник'))

const tg = useAsync((signal) => getTelegramLink(signal))
onMounted(() => void tg.run())

const appVersion = __APP_VERSION__

function go(path: string): void {
  router.push(path)
}
</script>

<template>
  <div class="profile">
    <h1 class="title">Профиль</h1>

    <Section v-if="me">
      <Cell label="Имя" :value="me.full_name" />
      <Cell label="Логин" :value="'@' + me.login" />
      <Cell label="Роль" :value="roleLabel" />
      <Cell label="Персона" :value="me.persona?.name ?? '—'" />
    </Section>

    <Section title="Telegram">
      <template v-if="tg.data.value && tg.data.value.linked">
        <Cell label="Аккаунт">
          <span v-if="tg.data.value.tg_username">@{{ tg.data.value.tg_username }}</span>
          <span v-else>привязан</span>
          <span v-if="tg.data.value.linked_at" class="hint">
            · привязан {{ formatDateTime(tg.data.value.linked_at) }}
          </span>
        </Cell>
        <Cell v-if="tg.data.value.bot_blocked">
          <span class="warn">
            Бот заблокирован — разблокируйте его, чтобы пользоваться кнопкой «Оплачено» в чате
          </span>
        </Cell>
      </template>
      <Cell v-else-if="tg.loading.value" value="Загрузка…" />
      <div class="unlink-hint hint">
        Отвязать Telegram можно командой /unlink в личке с ботом или в профиле на сайте
      </div>
    </Section>

    <Section v-if="isAdmin" title="Администрирование">
      <Cell clickable chevron @click="go('/admin/recurring')">Регулярные платежи</Cell>
      <Cell clickable chevron @click="go('/admin/users')">Сотрудники</Cell>
    </Section>

    <p class="version hint">Версия {{ appVersion }}</p>
  </div>
</template>

<style scoped>
.warn {
  display: block;
  color: var(--bone);
  font-size: var(--fs-small);
  border-left: 2px solid rgba(var(--violet), 0.9);
  padding-left: 10px;
  margin-top: 8px;
}
.unlink-hint {
  padding: var(--sp-4) 0;
}
.version {
  text-align: left;
  margin: var(--sp-8) 0;
  font-size: var(--fs-micro);
}
</style>
