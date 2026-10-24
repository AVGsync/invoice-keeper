<script setup lang="ts">
import type { Persona } from '~/types/api'

defineProps<{ items: Persona[], loading?: boolean }>()
defineEmits<{ add: [] }>()
</script>

<template>
  <aside class="card panel">
    <header class="head">
      <h2 class="title display">Персоны</h2>
      <span class="hint">на права не влияют</span>
    </header>

    <ul class="list">
      <li v-for="n in (loading ? 3 : 0)" :key="`s${n}`" class="item">
        <span class="stack"><UiSkeleton width="100px" /><UiSkeleton width="150px" height="10px" /></span>
      </li>
      <li v-for="item in (loading ? [] : items)" :key="item.id" class="item">
        <span>
          <span class="name">{{ item.name }}</span>
          <span v-if="item.description" class="description">{{ item.description }}</span>
        </span>
        <span class="count mono">{{ item.users_count ?? 0 }} польз.</span>
      </li>
      <li v-if="!loading && !items.length" class="item muted">Персон пока нет</li>
    </ul>

    <footer class="foot">
      <button type="button" class="add" @click="$emit('add')">+&nbsp; Добавить персону</button>
    </footer>
  </aside>
</template>

<style scoped>
.panel {
  align-self: flex-start;
  width: 320px;
}

.head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 56px;
  padding: 0 20px;
  border-bottom: 1px solid var(--line);
}

.title {
  font-size: 23px;
}

.hint {
  font-size: 12px;
  color: var(--ink-3);
}

.list {
  margin: 0;
  padding: 0 20px;
  list-style: none;
}

.item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 58px;
}

.item + .item {
  border-top: 1px solid var(--line);
}

.stack {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.name {
  display: block;
  font-weight: 500;
}

.description {
  font-size: 12px;
  color: var(--ink-3);
}

.count {
  font-size: 12px;
  color: var(--ink-2);
}

.foot {
  padding: 18px 20px 19px;
  border-top: 1px solid var(--line);
}

.add {
  padding: 0;
  background: none;
  border: 0;
  font-size: 13px;
  font-weight: 500;
  color: var(--accent-ink);
}
</style>
