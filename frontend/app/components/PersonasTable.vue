<script setup lang="ts">
import type { Persona } from '~/types/api'

defineProps<{ personas: Persona[], loading?: boolean }>()
defineEmits<{ edit: [persona: Persona] }>()
</script>

<template>
  <div class="card table">
    <div class="row head">
      <span>Название</span>
      <span>Описание</span>
      <span>Пользователей</span>
      <span>Создана</span>
      <span />
    </div>

    <template v-if="loading">
      <div v-for="n in 3" :key="n" class="row">
        <UiSkeleton width="120px" />
        <UiSkeleton width="220px" />
        <UiSkeleton width="16px" />
        <UiSkeleton width="70px" />
        <span />
      </div>
    </template>

    <div v-for="persona in (loading ? [] : personas)" :key="persona.id" class="row">
      <span class="name">{{ persona.name }}</span>
      <span :class="{ muted: !persona.description }">{{ persona.description || '—' }}</span>
      <span class="mono">{{ persona.users_count ?? 0 }}</span>
      <span class="mono soft">{{ formatDate(persona.created_at) }}</span>
      <button type="button" class="action soft" @click="$emit('edit', persona)">Изменить</button>
    </div>

    <UiState v-if="!loading && !personas.length">Персон пока нет. Создайте первую — например, «Бухгалтер».</UiState>
  </div>
</template>

<style scoped>
.table {
  font-size: 13px;
}

.row {
  display: grid;
  grid-template-columns: 220px 1fr 120px 110px 90px;
  align-items: center;
  height: 56px;
  margin: 0;
  padding: 0 20px;
  border-top: 1px solid var(--line);
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

.name {
  font-size: 14px;
  font-weight: 500;
}

.action {
  background: none;
  border: 0;
  padding: 0;
  font-size: 13px;
  text-align: right;
}
</style>
