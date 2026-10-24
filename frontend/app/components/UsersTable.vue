<script setup lang="ts">
import type { User } from '~/types/api'

defineProps<{ users: User[], loading?: boolean }>()
defineEmits<{ edit: [user: User] }>()
</script>

<template>
  <div class="card table">
    <div class="row head">
      <span>Пользователь</span>
      <span>Роль</span>
      <span>Персона</span>
      <span>Отвечает за</span>
      <span>Telegram</span>
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
        <UiSkeleton width="70px" />
        <span />
      </div>
    </template>

    <div v-for="user in (loading ? [] : users)" :key="user.id" class="row">
      <span>
        <span class="name">{{ user.full_name }}</span>
        <span class="login mono">{{ user.login }}</span>
      </span>
      <span>
        <span class="role mono" :class="user.role">{{ user.role }}</span>
      </span>
      <span v-if="user.persona" class="persona"><span class="dot" />{{ user.persona.name }}</span>
      <span v-else class="muted">—</span>
      <!-- backend has no payments or telegram yet -->
      <span class="muted">—</span>
      <span class="muted">—</span>
      <span class="mono soft">{{ formatDate(user.created_at) }}</span>
      <!-- admin comes from .env, can't be edited here -->
      <span v-if="user.role === 'admin'" class="action muted">через .env</span>
      <button v-else type="button" class="action soft" @click="$emit('edit', user)">Изменить</button>
    </div>
  </div>
</template>

<style scoped>
.table {
  font-size: 13px;
}

.row {
  display: grid;
  grid-template-columns: 210px 80px 140px 102px 72px 74px 1fr;
  align-items: center;
  height: 60px;
  padding: 0 20px;
  border-top: 1px solid var(--line);
  white-space: nowrap;
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

.stack {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.name {
  display: block;
  font-size: 14px;
  font-weight: 500;
}

.login {
  font-size: 12px;
  color: var(--ink-3);
}

.role {
  display: inline-block;
  height: 22px;
  padding: 0 8px;
  border-radius: 4px;
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

.persona {
  display: inline-flex;
  align-items: center;
  gap: 7px;
}

.dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--accent);
}

.action {
  background: none;
  border: 0;
  padding: 0;
  font-size: 13px;
  text-align: right;
}
</style>
