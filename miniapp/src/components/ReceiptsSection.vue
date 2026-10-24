<script setup lang="ts">
import { ref, watch } from 'vue'
import {
  addReceiptLink,
  deleteReceipt,
  listReceipts,
  receiptAccess,
  uploadReceipt,
} from '../api/endpoints'
import { humanError } from '../lib/humanError'
import { formatDate, today } from '../lib/dates'
import { alert as tgAlert, confirm as tgConfirm, haptic, openExternal } from '../telegram/webapp'
import type { Receipt } from '../api/types'
import Section from './Section.vue'

const props = defineProps<{ paymentId: string }>()

const MAX_BYTES = 10 * 1024 * 1024
const ACCEPT =
  'image/*,application/pdf,.pdf,.heic,.heif,.txt,.csv,.docx,.xlsx,.odt,.ods'

const items = ref<Receipt[]>([])
const loaded = ref(false)
const busy = ref(false)
const showLink = ref(false)
const linkUrl = ref('')
const linkTitle = ref('')
const fileInput = ref<HTMLInputElement | null>(null)

async function load(): Promise<void> {
  try {
    items.value = await listReceipts(props.paymentId)
  } catch {
    items.value = []
  } finally {
    loaded.value = true
  }
}

watch(() => props.paymentId, () => void load(), { immediate: true })

function size(n?: number): string {
  if (!n) return ''
  if (n < 1024) return `${n} Б`
  if (n < 1024 * 1024) return `${Math.round(n / 1024)} КБ`
  return `${(n / 1024 / 1024).toFixed(1).replace('.', ',')} МБ`
}

function host(url?: string): string {
  try {
    return new URL(url ?? '').hostname
  } catch {
    return url ?? ''
  }
}

function label(r: Receipt): string {
  return r.title || r.file_name || host(r.url) || 'Чек'
}

function meta(r: Receipt): string {
  const base = r.kind === 'file' ? size(r.size_bytes) : host(r.url)
  const date = formatDate(r.created_at.slice(0, 10), { today: today() })
  return [base, date].filter(Boolean).join(' · ')
}

async function run(action: () => Promise<void>): Promise<void> {
  if (busy.value) return
  busy.value = true
  try {
    await action()
    haptic.success()
  } catch (e) {
    haptic.error()
    await tgAlert(humanError(e))
  } finally {
    busy.value = false
  }
}

function pickFile(): void {
  fileInput.value?.click()
}

async function onFile(ev: Event): Promise<void> {
  const input = ev.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file) return
  if (file.size > MAX_BYTES) {
    await tgAlert('Файл больше 10 МБ')
    return
  }
  await run(async () => {
    await uploadReceipt(props.paymentId, file)
    await load()
  })
}

async function saveLink(): Promise<void> {
  const url = linkUrl.value.trim()
  if (!url) return
  await run(async () => {
    await addReceiptLink(props.paymentId, url, linkTitle.value.trim())
    linkUrl.value = ''
    linkTitle.value = ''
    showLink.value = false
    await load()
  })
}

async function open(r: Receipt): Promise<void> {
  await run(async () => {
    const access = await receiptAccess(r.id)
    openExternal(access.kind === 'file' ? location.origin + access.url : access.url)
  })
}

async function remove(r: Receipt): Promise<void> {
  if (!(await tgConfirm(`Удалить «${label(r)}»?`))) return
  await run(async () => {
    await deleteReceipt(r.id)
    await load()
  })
}
</script>

<template>
  <Section title="Чеки">
    <p v-if="loaded && !items.length" class="hint empty">Чеков пока нет</p>

    <div v-for="r in items" :key="r.id" class="rc">
      <button class="open row-active" type="button" :disabled="busy" @click="open(r)">
        <svg class="ico" viewBox="0 0 24 24" aria-hidden="true" focusable="false">
          <path v-if="r.kind === 'file'" d="M6 3h8l4 4v14H6zM14 3v4h4M9 12h6M9 16h6" />
          <path v-else d="M10 14a4 4 0 0 0 5.7 0l3-3a4 4 0 0 0-5.7-5.7l-1 1M14 10a4 4 0 0 0-5.7 0l-3 3a4 4 0 0 0 5.7 5.7l1-1" />
        </svg>
        <span class="txt">
          <span class="name">{{ label(r) }}</span>
          <span class="meta hint">{{ meta(r) }}</span>
        </span>
      </button>
      <button class="del" type="button" :disabled="busy" aria-label="Удалить чек" @click="remove(r)">
        <svg viewBox="0 0 24 24" aria-hidden="true" focusable="false">
          <path d="M5 7h14M10 7V4h4v3M7 7l1 13h8l1-13" />
        </svg>
      </button>
    </div>

    <div class="actions">
      <button class="btn btn-secondary" type="button" :disabled="busy" @click="pickFile">
        Прикрепить файл
      </button>
      <button class="btn btn-secondary" type="button" :disabled="busy" @click="showLink = !showLink">
        Добавить ссылку
      </button>
    </div>

    <form v-if="showLink" class="link-form" @submit.prevent="saveLink">
      <input
        v-model="linkUrl"
        type="url"
        inputmode="url"
        autocomplete="off"
        placeholder="https://…"
        aria-label="Ссылка на чек"
      />
      <input v-model="linkTitle" type="text" maxlength="120" placeholder="Подпись (необязательно)" />
      <button class="btn" type="submit" :disabled="busy || !linkUrl.trim()">Сохранить ссылку</button>
    </form>

    <input ref="fileInput" class="file" type="file" :accept="ACCEPT" @change="onFile" />
  </Section>
</template>

<style scoped>
.empty {
  margin: 0;
  padding: var(--sp-4) 0;
}
.rc {
  display: flex;
  align-items: center;
  gap: var(--sp-2);
  border-bottom: 1px solid var(--c-line);
}
.open {
  flex: 1;
  min-width: 0;
  display: flex;
  align-items: center;
  gap: var(--sp-3);
  padding: var(--sp-3) 0;
  min-height: 56px;
  border: 0;
  background: transparent;
  color: var(--bone-text);
  text-align: left;
}
.ico {
  flex: 0 0 22px;
  width: 22px;
  height: 22px;
  fill: none;
  stroke: var(--ash-text);
  stroke-width: 1.5;
  stroke-linecap: round;
  stroke-linejoin: round;
}
.txt {
  min-width: 0;
  display: flex;
  flex-direction: column;
}
.name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: var(--fs-body-l);
  font-weight: 500;
}
.meta {
  font-size: var(--fs-small);
}
.del {
  flex: 0 0 44px;
  width: 44px;
  height: 44px;
  display: grid;
  place-items: center;
  border: 0;
  border-radius: 50%;
  background: transparent;
  color: var(--muted);
}
.del svg {
  width: 20px;
  height: 20px;
  fill: none;
  stroke: currentColor;
  stroke-width: 1.5;
  stroke-linecap: round;
  stroke-linejoin: round;
}
.del:active {
  background: var(--press);
}
.actions {
  display: flex;
  flex-direction: column;
  gap: var(--sp-2);
  padding: var(--sp-4) 0;
}
.link-form {
  display: flex;
  flex-direction: column;
  gap: var(--sp-2);
  padding-bottom: var(--sp-4);
}
.file {
  display: none;
}
</style>
