<script setup lang="ts">
import type { User } from '~/types/api'

const props = defineProps<{ users: User[], loading?: boolean }>()
defineEmits<{ edit: [user: User] }>()

const { visible, hasMore, sentinel } = useLazyList(() => props.users)

// «Отвечает за»: the user's running recurring payments
const payments = usePayments()
onMounted(payments.load)
const responsibleFor = (id?: string) => payments.templates.value.filter(item => !item.ended && item.assigneeId === id)
const countText = (n: number) => `${n} ${plural(n, ['платёж', 'платежа', 'платежей'])}`

// the list of those payments in a small card under the count, on hover or focus.
// it's drawn over the page (Teleport), the table's round corners would cut it off
const shown = ref<{ id?: string, x: number, y: number }>()
function showList(user: User, e: Event) {
  const box = (e.currentTarget as HTMLElement).getBoundingClientRect()
  shown.value = { id: user.id, x: box.left, y: box.bottom + 6 }
}
</script>

<template>
  <div class="card table">
    <div class="row head">
      <span>Пользователь</span>
      <span>Роль</span>
      <span>Должность</span>
      <span>Отвечает за</span>
      <span>Telegram</span>
      <span>Почта</span>
      <span>Создан</span>
      <span />
    </div>

    <!-- grey placeholder rows until the list arrives -->
    <template v-if="loading">
      <div v-for="n in 4" :key="n" class="row">
        <span class="stack"><UiSkeleton width="140px" /><UiSkeleton width="60px" height="10px" /></span>
        <UiSkeleton width="44px" height="20px" />
        <UiSkeleton width="90px" />
        <UiSkeleton width="16px" />
        <UiSkeleton width="16px" />
        <UiSkeleton width="60px" />
        <UiSkeleton width="70px" />
        <span />
      </div>
    </template>

    <div v-for="(user, i) in (loading ? [] : visible)" :key="user.id" class="row rise" :style="{ '--i': i }">
      <span>
        <span class="name">{{ user.full_name }}</span>
        <span class="login mono">{{ user.login }}</span>
      </span>
      <span>
        <span class="role" :class="user.role">{{ roleName(user.role) }}</span>
      </span>
      <span v-if="user.persona">{{ user.persona.name }}</span>
      <span v-else class="muted">—</span>
      <span
        v-if="responsibleFor(user.id).length"
        class="count"
        tabindex="0"
        @mouseenter="showList(user, $event)"
        @mouseleave="shown = undefined"
        @focus="showList(user, $event)"
        @blur="shown = undefined"
      >{{ countText(responsibleFor(user.id).length) }}</span>
      <span v-else class="muted">—</span>
      <span
        v-if="user.telegram_linked"
        class="linked"
        :title="user.telegram_linked_at ? `Привязан ${formatDate(user.telegram_linked_at)}` : 'Привязан'"
      ><span class="dot" />{{ user.telegram_username ? `@${user.telegram_username}` : 'привязан' }}</span>
      <span v-else class="muted">—</span>
      <!-- the address itself isn't shown, only whether letters can reach the person -->
      <span
        v-if="user.email_linked"
        class="linked"
        :class="{ waiting: !user.email_verified }"
        :title="user.email_verified ? 'Почта подтверждена, письма приходят' : 'Почта указана, но не подтверждена кодом: письма не приходят'"
      ><span class="dot" />{{ user.email_verified ? 'подтверждена' : 'ждёт кода' }}</span>
      <span v-else class="muted" title="Почта для напоминаний не указана">—</span>
      <span class="mono soft">{{ formatDate(user.created_at) }}</span>
      <!-- admin comes from .env, can't be edited here -->
      <span v-if="user.role === 'admin'" class="action muted" title="Администратора меняют в настройках сервера">Не изменяется</span>
      <button v-else type="button" class="action soft" @click="$emit('edit', user)">Изменить</button>
    </div>
    <div v-if="hasMore" ref="sentinel" class="sentinel" />

    <Teleport to="body">
      <Transition name="menu">
        <div v-if="shown" class="card payments-list" :style="{ left: `${shown.x}px`, top: `${shown.y}px` }">
          <p class="kicker">Отвечает за</p>
          <div v-for="item in responsibleFor(shown.id)" :key="item.id" class="payment">
            <span class="payment-title">{{ item.title }}</span>
            <span class="mono payment-amount">{{ item.amount }}</span>
          </div>
        </div>
      </Transition>
    </Teleport>
  </div>
</template>

<style scoped>
/* linked telegram or email: green dot and a word, like the «привязан» state in Настройки */
.linked {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
  overflow: hidden;
  color: var(--green);
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* email saved but not confirmed yet */
.linked.waiting {
  color: var(--amber);
}

.linked .dot {
  flex-shrink: 0;
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: currentColor;
}

/* dashed underline: there is a list behind it */
.count {
  cursor: default;
  text-decoration: underline dotted var(--ink-3);
  text-underline-offset: 3px;
}

.count:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 2px;
  border-radius: 4px;
}

/* like the dropdown menus: surface, hairline, round corners, soft shadow */
.payments-list {
  position: fixed;
  z-index: 20;
  min-width: 220px;
  max-width: 320px;
  padding: 10px 12px;
  box-shadow: var(--shadow-pop);
  pointer-events: none;
  transform-origin: top left;
}

.payments-list .kicker {
  margin-bottom: 6px;
}

.payment {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  padding: 4px 0;
  font-size: 13px;
}

.payment + .payment {
  border-top: 1px solid var(--line);
}

.payment-title {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.payment-amount {
  flex-shrink: 0;
  color: var(--ink-2);
}

.sentinel {
  height: 1px;
}

/* a rounded card like the job titles next to it, rows separated by hairlines inside.
   overflow keeps the hover fill inside the round corners */
.table {
  flex: 1;
  overflow: hidden;
  font-size: 13px;
}

.row:not(.head):hover {
  background: var(--subtle);
}

.row {
  display: grid;
  /* every column has its least width and a share of what's left, so the table fills the card evenly */
  grid-template-columns:
    minmax(100px, 2fr) minmax(108px, 1.2fr) minmax(72px, 1.3fr) minmax(72px, 1fr)
    minmax(64px, 1.2fr) minmax(110px, 1.1fr) minmax(80px, 1fr) minmax(96px, 0.9fr);
  column-gap: 8px;
  align-items: center;
  height: 60px;
  padding: 0 16px;
  border-top: 1px solid var(--line);
  white-space: nowrap;
}

/* a cell may get narrower than its text, long names end with «…» */
.row > * {
  min-width: 0;
}

/* header: plain small text, the hairline under it does the separating */
.row.head {
  height: 40px;
  border-top: 0;
  font-size: 12px;
  font-weight: 500;
  color: var(--ink-3);
}

.stack {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.name {
  display: block;
  overflow: hidden;
  font-size: 14px;
  font-weight: 500;
  text-overflow: ellipsis;
}

.login {
  font-size: 12px;
  color: var(--ink-3);
}

.role {
  display: inline-block;
  height: 22px;
  padding: 0 9px;
  border-radius: 9999px;
  font-size: 11px;
  font-weight: 500;
  line-height: 22px;
}

.role.admin {
  background: var(--ink);
  color: var(--surface);
}

.role.user {
  background: var(--subtle);
  border: 1px solid var(--line);
  color: var(--ink-2);
  line-height: 20px;
}

.action {
  background: none;
  border: 0;
  padding: 0;
  font-size: 13px;
  text-align: right;
}

/* phones */
@media (max-width: 1240px) {
  .row.head {
    display: none;
  }

  /* a row is a card: name and role, position and responsibilities, Telegram and mail, date and action */
  .row {
    grid-template-columns: minmax(0, 1fr) auto;
    grid-template-areas:
      'user role'
      'persona count'
      'tg tg'
      'mail mail'
      'created action';
    height: auto;
    row-gap: 8px;
    padding: 14px 16px;
    white-space: normal;
  }

  .row > :nth-child(1) { grid-area: user; }
  .row > :nth-child(2) { grid-area: role; justify-self: end; }
  .row > :nth-child(3) { grid-area: persona; }
  .row > :nth-child(4) { grid-area: count; justify-self: end; }
  .row > :nth-child(5) { grid-area: tg; }
  .row > :nth-child(6) { grid-area: mail; }
  .row > :nth-child(7) { grid-area: created; align-self: center; }
  .row > :nth-child(8) { grid-area: action; justify-self: end; }

  .row:not(.head) > :nth-child(3)::before,
  .row:not(.head) > :nth-child(4)::before,
  .row:not(.head) > :nth-child(5)::before,
  .row:not(.head) > :nth-child(6)::before,
  .row:not(.head) > :nth-child(7)::before {
    margin-right: 6px;
    font-size: 12px;
    color: var(--ink-3);
  }

  .row:not(.head) > :nth-child(3)::before { content: 'Должность'; }
  .row:not(.head) > :nth-child(4)::before { content: 'Отвечает за'; }
  .row:not(.head) > :nth-child(5)::before { content: 'Telegram'; }
  .row:not(.head) > :nth-child(6)::before { content: 'Почта'; }
  .row:not(.head) > :nth-child(7)::before { content: 'Создан'; }

  .action {
    height: 36px;
    padding: 0 14px;
    border: 1px solid var(--line);
    border-radius: 9999px;
  }
}
</style>
