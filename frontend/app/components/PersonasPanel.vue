<script setup lang="ts">
import type { Persona } from '~/types/api'

const props = defineProps<{ items: Persona[], loading?: boolean }>()
defineEmits<{ add: [], edit: [persona: Persona] }>()

const { visible, hasMore, sentinel } = useLazyList(() => props.items)
</script>

<template>
  <!-- the same rows as the users table next to it, in a rounded card -->
  <aside class="card table panel">
    <div class="row head">
      <span>Должность</span>
      <span class="right">Польз.</span>
      <span />
    </div>

    <template v-if="loading">
      <div v-for="n in 3" :key="n" class="row">
        <span class="stack"><UiSkeleton width="100px" /><UiSkeleton width="150px" height="10px" /></span>
        <UiSkeleton width="16px" class="right-skeleton" />
        <span />
      </div>
    </template>

    <div v-for="(item, i) in (loading ? [] : visible)" :key="item.id" class="row item rise" :style="{ '--i': i }">
      <span class="text">
        <span class="name">{{ item.name }}</span>
        <span v-if="item.description" class="description" :title="item.description">{{ item.description }}</span>
      </span>
      <span class="mono right">{{ item.users_count ?? 0 }}</span>
      <button type="button" class="action" @click="$emit('edit', item)">Изменить</button>
    </div>

    <div v-if="!loading && !items.length" class="row empty">Должностей пока нет</div>
    <div v-if="hasMore" ref="sentinel" class="sentinel" />

    <div class="row foot">
      <button type="button" class="add" @click="$emit('add')">+&nbsp; Добавить должность</button>
    </div>
  </aside>
</template>

<style scoped>
.table {
  flex-shrink: 0;
  width: 320px;
  padding: 0 16px;
  font-size: 13px;
}

.row {
  display: grid;
  grid-template-columns: 1fr 48px 76px;
  align-items: center;
  gap: 12px;
  height: 60px;
  border-top: 1px solid var(--line);
}

/* header: plain small text, the hairline under it does the separating */
.row.head {
  height: 40px;
  border-top: 0;
  font-size: 12px;
  font-weight: 500;
  color: var(--ink-3);
}

.text {
  min-width: 0;
}

.name {
  display: block;
  font-size: 14px;
  font-weight: 500;
}

/* one line, the full text is in the tooltip */
.description {
  display: block;
  overflow: hidden;
  font-size: 12px;
  color: var(--ink-3);
  text-overflow: ellipsis;
  white-space: nowrap;
}

.stack {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.right-skeleton {
  justify-self: end;
}

.empty {
  display: flex;
  color: var(--ink-3);
}

.sentinel {
  height: 1px;
}

.action {
  justify-self: end;
  padding: 0;
  background: none;
  border: 0;
  font-size: 13px;
  color: var(--ink-2);
}

.action:hover {
  color: var(--ink);
}

.foot {
  display: flex;
}

.add {
  padding: 0;
  background: none;
  border: 0;
  font-size: 13px;
  font-weight: 500;
  color: var(--accent-ink);
}

/* phones */
@media (max-width: 1240px) {
  .table {
    width: 100%;
  }
}
</style>
