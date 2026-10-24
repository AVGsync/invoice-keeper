<script setup lang="ts">
import type { Template } from '~/utils/demo'

defineProps<{ templates: Template[], demo?: boolean }>()
defineEmits<{ stop: [template: Template] }>()
const { isAdmin } = useRole()
</script>

<template>
  <div class="card table" :class="{ admin: isAdmin }">
    <div class="row head">
      <span class="mono muted">№</span>
      <span>Название</span>
      <span class="right">Сумма</span>
      <span class="currency">Валюта</span>
      <span>Периодичность</span>
      <span>Ответственный</span>
      <span>Следующий срок</span>
      <span>Статус</span>
      <span v-if="isAdmin" />
    </div>

    <div v-for="(item, i) in templates" :key="item.title" class="row" :class="{ ended: item.ended }">
      <span class="mono muted small">{{ String(i + 1).padStart(2, '0') }}</span>
      <span>
        <span class="title">{{ item.title }}</span>
        <span class="small muted">{{ item.category }}</span>
      </span>
      <span class="mono strong right dim">{{ item.amount }}</span>
      <span class="mono small muted currency">{{ item.currency }}</span>
      <span>
        <span class="dim">{{ item.frequency }}</span>
        <span class="small muted">{{ item.day }}</span>
      </span>
      <PersonCell :person="item.who" />
      <span class="mono dim">{{ item.nextDue ?? '—' }}</span>
      <span v-if="item.ended" class="muted">Прекращён</span>
      <UiStatus v-else status="active" />
      <!-- «Прекратить» works in demo mode, «История» needs the backend either way -->
      <template v-if="isAdmin">
        <button v-if="!item.ended && demo" type="button" class="action" @click="$emit('stop', item)">Прекратить</button>
        <button v-else type="button" class="action text-action soon" disabled :title="soonHint">
          {{ item.ended ? 'История' : 'Прекратить' }}
        </button>
      </template>
    </div>
  </div>
</template>

<style scoped>
.table {
  position: relative;
  font-size: 13px;
}

.table::before {
  content: '';
  position: absolute;
  top: 0;
  bottom: 0;
  left: 44px;
  width: 1px;
  background: color-mix(in srgb, var(--accent) 35%, transparent);
}

.row {
  display: grid;
  grid-template-columns: 36px 234px 130px 90px 180px 160px 120px 1fr;
  align-items: center;
  height: 56px;
  padding: 0 20px;
  border-top: 1px solid var(--line);
  white-space: nowrap;
}

.admin .row {
  grid-template-columns: 36px 234px 130px 90px 180px 160px 120px 1fr 90px;
}

.row.head {
  height: 40px;
  border-top: 0;
  border-radius: 10px 10px 0 0;
  background: var(--subtle);
  font-size: 12px;
  font-weight: 500;
  color: var(--ink-2);
}

.title {
  display: block;
  font-size: 14px;
  font-weight: 500;
}

.small {
  display: block;
  font-size: 12px;
}

.strong {
  font-weight: 500;
}

.currency {
  padding-left: 20px;
}

/* stopped payment: everything grey except who pays */
.ended .title,
.ended .dim {
  color: var(--ink-3);
}

.action {
  background: none;
  border: 0;
  padding: 0;
  font-size: 13px;
  color: var(--ink-2);
  text-align: right;
}
</style>
