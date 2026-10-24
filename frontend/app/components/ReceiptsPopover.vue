<!-- receipts of one payment period: the list (a click opens or downloads one), adding a file or a link,
     removing one. a file can also be dropped onto the card -->
<script setup lang="ts">
import type { Receipt } from '~/types/api'
import type { Period } from '~/utils/periods'

const props = defineProps<{ period: Period, canEdit: boolean }>()
const emit = defineEmits<{ close: [] }>()

const receipts = useReceipts()
const list = computed(() => receipts.byPeriod.value[props.period.id])
const loadError = ref('')

async function load() {
  loadError.value = ''
  try {
    await receipts.load(props.period.id)
  }
  catch (e) {
    loadError.value = apiError(e)
  }
}
onMounted(load)

const root = ref<HTMLElement>()
useClickOutside(() => root.value?.parentElement, () => emit('close'))

const busy = ref(false)
const error = ref('')

async function run(action: () => Promise<unknown>) {
  busy.value = true
  error.value = ''
  try {
    await action()
  }
  catch (e) {
    error.value = apiError(e)
  }
  finally {
    busy.value = false
  }
}

// files: the button opens the picker, several can go at once
const picker = ref<HTMLInputElement>()
async function uploadFiles(files: FileList | File[] | null | undefined) {
  if (!files?.length) return
  await run(async () => {
    for (const file of Array.from(files)) await receipts.upload(props.period.id, file)
  })
  if (picker.value) picker.value.value = ''
}

// a file dragged over the card lights it up, dropping it uploads
const dragging = ref(false)
function drop(e: DragEvent) {
  dragging.value = false
  if (props.canEdit) uploadFiles(e.dataTransfer?.files)
}

// a link: a small form under the list
const linkOpen = ref(false)
const link = reactive({ url: '', title: '' })
async function addLink() {
  if (!link.url.trim()) return
  await run(() => receipts.addLink(props.period.id, link.url.trim(), link.title.trim()))
  if (!error.value) {
    link.url = ''
    link.title = ''
    linkOpen.value = false
  }
}

function openReceipt(receipt: Receipt) {
  receipts.open(receipt).catch((e) => { error.value = apiError(e) })
}

// removing can't be undone, so the first click only asks
const confirming = ref<string>()
function remove(receipt: Receipt) {
  if (confirming.value !== receipt.id) {
    confirming.value = receipt.id
    return
  }
  confirming.value = undefined
  run(() => receipts.remove(props.period.id, receipt))
}

const nameOf = (receipt: Receipt) => receipt.title || receipt.file_name || receipt.url?.replace(/^https?:\/\//, '') || 'Чек'
function sizeOf(bytes?: number) {
  if (!bytes) return ''
  if (bytes < 1024 * 1024) return `${Math.max(1, Math.round(bytes / 1024))} КБ`
  return `${(bytes / 1024 / 1024).toFixed(1).replace('.', ',')} МБ`
}
const metaOf = (receipt: Receipt) => [
  receipt.kind === 'link' ? 'ссылка' : sizeOf(receipt.size_bytes),
  receipt.uploaded_by_name,
  receipt.created_at ? formatDate(receipt.created_at) : '',
].filter(Boolean).join(' · ')

// the same formats the server takes
const accept = '.pdf,.jpg,.jpeg,.png,.gif,.webp,.heic,.txt,.csv,.docx,.xlsx,.odt,.ods,application/pdf,image/*'
</script>

<template>
  <div
    ref="root"
    class="card popover"
    :class="{ dragging }"
    @click.stop
    @dragover.prevent="canEdit && (dragging = true)"
    @dragleave.self="dragging = false"
    @drop.prevent="drop"
  >
    <h3 class="title display">Чеки</h3>
    <p class="meta">{{ period.title }} · {{ period.amount }} · срок {{ period.due }}</p>

    <UiState v-if="loadError" kind="error" class="state" @retry="load">{{ loadError }}</UiState>

    <div v-else-if="!list" class="items">
      <div v-for="n in 2" :key="n" class="item">
        <UiSkeleton width="28px" height="28px" round />
        <span class="stack"><UiSkeleton width="150px" /><UiSkeleton width="100px" height="10px" /></span>
      </div>
    </div>

    <p v-else-if="!list.length" class="empty">{{ canEdit ? 'Чеков пока нет. Загрузите файл или перетащите его сюда.' : 'Чеков пока нет.' }}</p>

    <TransitionGroup v-else tag="div" name="item" class="items">
      <div v-for="receipt in list" :key="receipt.id" class="item">
        <button type="button" class="open" :title="receipt.kind === 'link' ? 'Открыть ссылку' : 'Открыть или скачать'" @click="openReceipt(receipt)">
          <span class="kind"><UiIcon :name="receipt.kind === 'link' ? 'link' : 'file'" :size="15" /></span>
          <span class="text">
            <span class="name">{{ nameOf(receipt) }}</span>
            <span class="small">{{ metaOf(receipt) }}</span>
          </span>
        </button>
        <button
          v-if="canEdit"
          type="button"
          class="remove"
          :class="{ confirm: confirming === receipt.id }"
          :disabled="busy"
          @click="remove(receipt)"
          @blur="confirming = undefined"
        >
          {{ confirming === receipt.id ? 'Удалить?' : '×' }}
        </button>
      </div>
    </TransitionGroup>

    <p v-if="error" class="hint error-text">{{ error }}</p>

    <template v-if="canEdit">
      <Transition name="menu">
        <form v-if="linkOpen" class="link-form" @submit.prevent="addLink">
          <UiInput v-model="link.url" placeholder="https://…" aria-label="Ссылка на чек" maxlength="2048" />
          <UiInput v-model="link.title" placeholder="Подпись, необязательно" aria-label="Подпись" maxlength="120" />
        </form>
      </Transition>

      <div class="buttons">
        <template v-if="linkOpen">
          <UiButton size="md" @click="linkOpen = false">Отмена</UiButton>
          <UiButton size="md" variant="primary" :disabled="busy || !link.url.trim()" @click="addLink">Прикрепить</UiButton>
        </template>
        <template v-else>
          <UiButton size="md" :disabled="busy" @click="linkOpen = true"><UiIcon name="link" :size="14" />Ссылка</UiButton>
          <UiButton size="md" variant="primary" :disabled="busy" @click="picker?.click()">
            <UiIcon name="upload" :size="14" />{{ busy ? 'Загружаем…' : 'Загрузить файл' }}
          </UiButton>
        </template>
      </div>
      <input ref="picker" type="file" class="picker" :accept="accept" multiple @change="uploadFiles(($event.target as HTMLInputElement).files)">
    </template>
  </div>
</template>

<style scoped>
.popover {
  width: 340px;
  padding: 16px 18px 18px;
  box-shadow: var(--shadow-pop);
  transition: border-color 0.2s ease, box-shadow 0.3s var(--ease-out);
}

/* a file is being dragged over: the card says «drop it here» */
.popover.dragging {
  border-color: var(--accent);
  box-shadow: var(--shadow-pop), 0 0 0 4px var(--accent-bg);
}

.title {
  font-size: 16px;
  line-height: 24px;
}

.meta {
  margin: 2px 0 10px;
  font-size: 12px;
  color: var(--ink-3);
}

.items {
  display: flex;
  flex-direction: column;
  gap: 2px;
  margin: 0 -8px 10px;
}

.item {
  display: flex;
  align-items: center;
  gap: 4px;
}

.open {
  display: flex;
  flex: 1;
  align-items: center;
  gap: 10px;
  min-width: 0;
  padding: 6px 8px;
  background: none;
  border: 0;
  border-radius: 10px;
  text-align: left;
  transition: background-color 0.2s ease;
}

.open:hover {
  background: var(--subtle);
}

.kind {
  display: grid;
  flex-shrink: 0;
  place-items: center;
  width: 28px;
  height: 28px;
  background: var(--accent-bg);
  border-radius: 8px;
  color: var(--accent-ink);
  transition: transform 0.35s var(--ease-spring);
}

.open:hover .kind {
  transform: scale(1.08) rotate(-4deg);
}

.text {
  min-width: 0;
}

.name,
.small {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.name {
  font-size: 13px;
  font-weight: 500;
}

.small {
  font-size: 12px;
  color: var(--ink-3);
}

.remove {
  flex-shrink: 0;
  min-width: 28px;
  height: 28px;
  padding: 0 8px;
  background: none;
  border: 0;
  border-radius: 9999px;
  font-size: 15px;
  color: var(--ink-3);
  transition: background-color 0.2s ease, color 0.2s ease;
}

.remove:hover {
  background: var(--subtle);
  color: var(--ink);
}

.remove.confirm {
  font-size: 12px;
  font-weight: 500;
  color: var(--red);
}

.stack {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.empty {
  margin: 4px 0 12px;
  font-size: 13px;
  color: var(--ink-2);
}

.hint {
  margin: 0 0 10px;
  font-size: 12px;
}

.state {
  margin-bottom: 10px;
}

.link-form {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin-bottom: 10px;
}

.buttons {
  display: grid;
  grid-template-columns: 112px 1fr;
  gap: 8px;
}

.buttons :deep(.btn) {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
}

.picker {
  display: none;
}

/* a new receipt slides in, a removed one folds away */
.item-enter-active,
.item-leave-active {
  transition: opacity 0.3s ease, transform 0.4s var(--ease-out);
}

.item-enter-from,
.item-leave-to {
  opacity: 0;
  transform: translateY(-6px);
}
</style>
