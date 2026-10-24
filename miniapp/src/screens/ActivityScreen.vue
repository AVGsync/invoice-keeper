<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { listActivity } from '../api/endpoints'
import { humanError } from '../lib/humanError'
import { formatDateTime } from '../lib/dates'
import type { AuditItem } from '../api/types'
import Chips from '../components/Chips.vue'
import Section from '../components/Section.vue'
import Skeleton from '../components/Skeleton.vue'
import EmptyState from '../components/EmptyState.vue'

const PAGE = 30
const route = useRoute()
const all = computed(() => route.meta.all === true)

const channel = ref('all')
const items = ref<AuditItem[]>([])
const total = ref(0)
const loading = ref(false)
const loaded = ref(false)
const error = ref('')

const CHANNELS: Record<string, string> = { web: 'Веб', miniapp: 'Mini App', bot: 'Бот', system: 'Система' }

async function load(reset: boolean): Promise<void> {
  if (loading.value) return
  loading.value = true
  error.value = ''
  try {
    const res = await listActivity(all.value ? 'all' : 'me', {
      limit: PAGE,
      offset: reset ? 0 : items.value.length,
      channel: channel.value === 'all' ? undefined : channel.value,
    })
    items.value = reset ? res.items : [...items.value, ...res.items]
    total.value = res.total
    loaded.value = true
  } catch (e) {
    error.value = humanError(e)
  } finally {
    loading.value = false
  }
}

function setChannel(v: string): void {
  channel.value = v
  void load(true)
}

function who(it: AuditItem): string {
  return it.actor?.full_name || (it.channel === 'system' ? 'Система' : 'Удалённый пользователь')
}

onMounted(() => void load(true))
</script>

<template>
  <div class="activity">
    <h1 class="title">{{ all ? 'Журнал действий' : 'Мои действия' }}</h1>

    <Chips
      :model-value="channel"
      :options="[
        { value: 'all', label: 'Все' },
        { value: 'web', label: 'Веб' },
        { value: 'miniapp', label: 'Mini App' },
        { value: 'bot', label: 'Бот' },
      ]"
      @update:model-value="setChannel"
    />

    <Skeleton v-if="!loaded && !error" :rows="6" />
    <p v-else-if="error && !loaded" class="form-error">{{ error }}</p>
    <EmptyState v-else-if="!items.length" text="Записей пока нет" />
    <template v-else>
      <Section>
        <div v-for="it in items" :key="it.id" class="row">
          <div class="top">
            <span class="act">{{ it.action_label }}</span>
            <span v-if="it.outcome === 'failure'" class="fail">не удалось</span>
          </div>
          <div class="sub hint">
            <span v-if="all">{{ who(it) }} · </span>{{ CHANNELS[it.channel] ?? it.channel }} ·
            {{ formatDateTime(it.created_at) }}
          </div>
        </div>
      </Section>
      <button
        v-if="items.length < total"
        class="btn btn-secondary btn-block more"
        type="button"
        :disabled="loading"
        @click="load(false)"
      >
        Показать ещё ({{ total - items.length }})
      </button>
    </template>
  </div>
</template>

<style scoped>
.row {
  padding: var(--sp-3) 0;
  border-bottom: 1px solid var(--c-line);
}
.top {
  display: flex;
  justify-content: space-between;
  gap: var(--sp-3);
}
.act {
  font-size: var(--fs-body-l);
  font-weight: 500;
}
.fail {
  font-size: var(--fs-caption);
  color: var(--red);
  white-space: nowrap;
}
.sub {
  font-size: var(--fs-small);
  margin-top: 2px;
}
.more {
  margin-top: var(--sp-4);
}
</style>
