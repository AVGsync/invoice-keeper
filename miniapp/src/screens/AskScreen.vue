<script setup lang="ts">
import { ref, computed, nextTick, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { assistant, assistantState } from '../stores/assistant'
import { isAdmin } from '../stores/session'
import { codeMessage } from '../api/ask'
import { useMainButton } from '../composables/useMainButton'
import Chips from '../components/Chips.vue'
import OccurrenceListBlock from '../components/blocks/OccurrenceListBlock.vue'
import CurrencyTotalsBlock from '../components/blocks/CurrencyTotalsBlock.vue'
import PaymentListBlock from '../components/blocks/PaymentListBlock.vue'
import PeopleListBlock from '../components/blocks/PeopleListBlock.vue'
import type {
  AssistantBlock,
  OccurrenceListBlock as OccBlock,
  CurrencyTotalsBlock as CurBlock,
  PaymentListBlock as PayBlock,
  PeopleListBlock as PplBlock,
} from '../api/types'

const router = useRouter()
const text = ref('')
const busy = computed(() => assistantState.history.some((e) => e.busy))
const feedEl = ref<HTMLElement | null>(null)
let controller: AbortController | null = null

const examples = computed(() => assistant.status.examples)
const canSend = computed(() => {
  const t = text.value.trim()
  return t.length >= 1 && t.length <= 500 && !busy.value
})

async function send(): Promise<void> {
  const q = text.value.trim()
  if (q.length < 1 || q.length > 500 || busy.value) return
  text.value = ''
  controller?.abort()
  controller = new AbortController()
  const p = assistant.ask(q, controller.signal)
  await nextTick()
  scrollToBottom()
  await p
  await nextTick()
  scrollToBottom()
}

function scrollToBottom(): void {
  const el = feedEl.value
  if (el) el.scrollTop = el.scrollHeight
}

function fillExample(e: string): void {
  text.value = e
}
function askFollowup(q: string): void {
  text.value = q
  void send()
}
function openOcc(id: string): void {
  router.push(`/occ/${id}`)
}
// payment_list — это регулярные платежи (шаблоны), а не периоды.
// У админа ведём в список шаблонов с подсветкой, у пользователя — в периоды этого шаблона.
function openPayment(id: string): void {
  if (isAdmin.value) router.push({ path: '/admin/recurring', query: { focus: id } })
  else router.push({ path: '/payments', query: { rec: id } })
}

function blockType(b: AssistantBlock): string {
  return (b as { type: string }).type
}

useMainButton({
  text: 'Спросить',
  enabled: canSend,
  loading: busy,
  onClick: send,
})

onUnmounted(() => controller?.abort())
</script>

<template>
  <div class="ask">
    <h1 class="title">Спросить</h1>

    <Chips
      v-if="examples.length && !assistantState.history.length"
      :model-value="''"
      :options="examples.map((e) => ({ value: e, label: e }))"
      @update:model-value="fillExample"
    />

    <div ref="feedEl" class="feed">
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

    <div class="composer">
      <textarea
        v-model="text"
        maxlength="500"
        rows="2"
        placeholder="Спросите про платежи…"
        @keydown.enter.exact.prevent="send"
      ></textarea>
      <div class="counter hint">{{ text.length }}/500</div>
    </div>
  </div>
</template>

<style scoped>
.title {
  font-size: 22px;
  margin: 16px 0 12px;
}
.feed {
  display: flex;
  flex-direction: column;
  gap: 16px;
  padding-bottom: 16px;
}
.exchange {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.bubble.q {
  align-self: flex-end;
  background: var(--c-primary);
  color: var(--c-primary-text);
  padding: 10px 14px;
  border-radius: 16px 16px 4px 16px;
  max-width: 85%;
}
.steps {
  list-style: none;
  margin: 0;
  padding: 0 4px;
  color: var(--c-hint);
  font-size: 14px;
}
.steps li {
  padding: 2px 0;
}
.steps li.spinner::after {
  content: ' …';
}
.answer {
  white-space: pre-wrap;
  padding: 4px 4px;
}
.err-card {
  background: color-mix(in srgb, var(--c-destructive) 12%, var(--c-bg));
  color: var(--c-destructive);
  padding: 12px 14px;
  border-radius: var(--radius);
}
.meta {
  color: var(--c-hint);
  font-size: 12px;
  padding: 0 4px;
}
.composer {
  position: sticky;
  bottom: 0;
  background: var(--c-bg);
  padding: 8px 0 4px;
}
.counter {
  text-align: right;
  margin-top: 2px;
}
</style>
