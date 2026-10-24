<script setup lang="ts">
import { ref, computed, nextTick, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { assistant, assistantState } from '../stores/assistant'
import { isAdmin } from '../stores/session'
import { codeMessage } from '../api/ask'
import { confirm, alert } from '../telegram/webapp'
import { humanError } from '../lib/humanError'
import Chips from '../components/Chips.vue'
import Composer from '../components/Composer.vue'
import OccurrenceListBlock from '../components/blocks/OccurrenceListBlock.vue'
import CurrencyTotalsBlock from '../components/blocks/CurrencyTotalsBlock.vue'
import PaymentListBlock from '../components/blocks/PaymentListBlock.vue'
import PeopleListBlock from '../components/blocks/PeopleListBlock.vue'
import TableBlock from '../components/blocks/TableBlock.vue'
import type {
  AssistantBlock,
  OccurrenceListBlock as OccBlock,
  CurrencyTotalsBlock as CurBlock,
  PaymentListBlock as PayBlock,
  PeopleListBlock as PplBlock,
  TableBlock as TblBlock,
} from '../api/types'

const router = useRouter()
const text = ref('')
const busy = computed(() => assistantState.history.some((e) => e.busy))
const feedEl = ref<HTMLElement | null>(null)
const composerEl = ref<InstanceType<typeof Composer> | null>(null)
let controller: AbortController | null = null

const examples = computed(() => assistant.status.examples)
const hasHistory = computed(() => assistantState.history.length > 0)

async function send(): Promise<void> {
  const q = text.value.trim()
  if (q.length < 1 || q.length > 500 || busy.value) return
  text.value = ''
  composerEl.value?.focus()
  controller?.abort()
  controller = new AbortController()
  const p = assistant.ask(q, controller.signal)
  await nextTick()
  scrollToBottom()
  await p
  await nextTick()
  scrollToBottom()
}

// Нажата кнопка-стоп во время стрима: прерываем запрос (обмен станет «Остановлено»).
function stop(): void {
  controller?.abort()
}

function scrollToBottom(): void {
  const el = feedEl.value
  if (el) el.scrollTop = el.scrollHeight
}

function fillExample(e: string): void {
  text.value = e
  composerEl.value?.focus()
}
function askFollowup(q: string): void {
  text.value = q
  void send()
}
function openOcc(id: string): void {
  router.push(`/occ/${id}`)
}
// payment_list — это регулярные платежи (шаблоны), а не периоды.
function openPayment(id: string): void {
  if (isAdmin.value) router.push({ path: '/admin/recurring', query: { focus: id } })
  else router.push({ path: '/payments', query: { rec: id } })
}

function blockType(b: AssistantBlock): string {
  return (b as { type: string }).type
}

// «Новый диалог»: ассистент забудет прошлые вопросы.
async function newConversation(): Promise<void> {
  const ok = await confirm('Начать новый диалог? Ассистент забудет прошлые вопросы.')
  if (!ok) return
  try {
    await assistant.resetConversation()
  } catch (e) {
    await alert(humanError(e))
  }
}

onMounted(() => {
  void assistant.loadHistory()
})
onUnmounted(() => controller?.abort())
</script>

<template>
  <div class="ask">
    <div class="title-row">
      <h1 class="title">Спросить</h1>
      <button
        v-if="hasHistory"
        type="button"
        class="newchat"
        aria-label="Новый диалог"
        @click="newConversation"
      >
        <svg viewBox="0 0 24 24" aria-hidden="true" focusable="false">
          <path
            d="M12 5v14M5 12h14"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            stroke-linecap="round"
          />
        </svg>
      </button>
    </div>

    <Chips
      v-if="examples.length && !assistantState.history.length"
      :model-value="''"
      :options="examples.map((e) => ({ value: e, label: e }))"
      @update:model-value="fillExample"
    />

    <div ref="feedEl" class="feed" :aria-busy="busy || undefined">
      <div v-for="(ex, i) in assistantState.history" :key="i" class="exchange">
        <div class="bubble q">{{ ex.q }}</div>

        <ul v-if="ex.steps.length" class="steps">
          <li v-for="(s, si) in ex.steps" :key="si" :class="{ spinner: ex.busy && si === ex.steps.length - 1 }">
            {{ s.label }}
          </li>
        </ul>

        <template v-for="(b, bi) in ex.blocks" :key="bi">
          <OccurrenceListBlock
            v-if="blockType(b) === 'occurrence_list'"
            :block="b as OccBlock"
            @open="openOcc"
          />
          <CurrencyTotalsBlock
            v-else-if="blockType(b) === 'currency_totals'"
            :block="b as CurBlock"
          />
          <PaymentListBlock
            v-else-if="blockType(b) === 'payment_list'"
            :block="b as PayBlock"
            @open="openPayment"
          />
          <PeopleListBlock v-else-if="blockType(b) === 'people_list'" :block="b as PplBlock" />
          <TableBlock v-else-if="blockType(b) === 'table'" :block="b as TblBlock" />
        </template>

        <div v-if="ex.text" class="answer">{{ ex.text }}</div>

        <div v-if="ex.status === 'error'" class="err-card">
          {{ ex.error ? codeMessage(ex.error) : 'Ошибка' }}
        </div>

        <Chips
          v-if="ex.followups.length"
          :model-value="''"
          :options="ex.followups.map((f) => ({ value: f, label: f }))"
          @update:model-value="askFollowup"
        />

        <div v-if="isAdmin && ex.meta" class="meta">
          <span v-if="ex.meta.model">{{ ex.meta.model }}</span>
          <span v-if="ex.meta.tokens"> · {{ ex.meta.tokens }} ток.</span>
          <span v-if="ex.meta.cost_usd"> · ${{ ex.meta.cost_usd }}</span>
        </div>
      </div>
    </div>

    <Composer
      ref="composerEl"
      v-model="text"
      :busy="busy"
      :maxlength="500"
      @submit="send"
      @stop="stop"
    />
  </div>
</template>

<style scoped>
.title-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--sp-3);
  margin: var(--sp-6) 0 var(--sp-5);
}
.title-row .title {
  margin: 0;
}
.newchat {
  flex: 0 0 auto;
  width: 40px;
  height: 40px;
  border: 1px solid var(--line-strong);
  border-radius: 50%;
  background: var(--glass);
  color: var(--bone-text);
  display: flex;
  align-items: center;
  justify-content: center;
  transition:
    background-color var(--dur-press) var(--ease-press),
    transform var(--dur-press) var(--ease-press);
}
.newchat svg {
  width: 20px;
  height: 20px;
}
.newchat:active {
  transform: scale(0.92);
  background: var(--press);
}
.feed {
  display: flex;
  flex-direction: column;
  gap: var(--sp-7);
  /* поле композера фиксировано снизу — освобождаем под него место */
  padding-bottom: calc(var(--composer-h, 56px) + 24px);
}
.exchange {
  animation: rise 0.32s var(--ease-out) both;
  display: flex;
  flex-direction: column;
  gap: var(--sp-3);
}
.bubble.q {
  transform-origin: bottom right;
  animation: bubble 0.28s var(--ease-out) both;
  align-self: flex-end;
  background: var(--graphite);
  border: 1px solid var(--line);
  color: var(--bone-text);
  padding: 12px 16px;
  border-radius: 20px 20px 6px 20px;
  max-width: 85%;
  font-size: var(--fs-body);
}
.steps {
  list-style: none;
  margin: 0;
  padding: 0 0 0 var(--sp-3);
  border-left: 1px solid var(--c-line);
  color: var(--fog);
  font-size: var(--fs-small);
}
.steps li {
  padding: 2px 0;
  animation: rise 0.24s var(--ease-out) both;
}
@keyframes bubble {
  from {
    opacity: 0;
    transform: scale(0.96) translateY(6px);
  }
}
.steps li.spinner::after {
  content: ' …';
  animation: blink 1.2s var(--ease) infinite alternate;
}
@keyframes blink {
  from {
    opacity: 0.3;
  }
  to {
    opacity: 1;
  }
}
@media (prefers-reduced-motion: reduce) {
  .steps li.spinner::after {
    animation: none;
  }
}
.answer {
  font-size: var(--fs-body-l);
  line-height: 1.45;
  white-space: pre-wrap;
  padding: 0;
}
.err-card {
  background: var(--graphite);
  border: 1px solid var(--line-strong);
  border-radius: var(--radius);
  color: var(--bone);
  padding: var(--sp-4) var(--sp-5);
}
.meta {
  color: var(--fog);
  font-size: var(--fs-micro);
  font-variant-numeric: tabular-nums;
  padding: 0;
}
.ask :deep(.chip) {
  color: var(--bone);
}
</style>
