<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { users, usersState } from '../../stores/users'
import { useMainButton } from '../../composables/useMainButton'
import Section from '../../components/Section.vue'
import Skeleton from '../../components/Skeleton.vue'
import ErrorState from '../../components/ErrorState.vue'

const router = useRouter()

onMounted(() => {
  if (!usersState.loaded) void users.load()
})

const firstLoad = computed(() => usersState.loading && !usersState.loaded)

useMainButton({
  text: 'Добавить сотрудника',
  onClick: () => {
    void router.push('/admin/users/new')
  },
})
</script>

<template>
  <div class="users">
    <h1 class="title">Сотрудники</h1>

    <Skeleton v-if="firstLoad" :rows="5" />
    <ErrorState
      v-else-if="usersState.error && !usersState.loaded"
      :error="usersState.error"
      @retry="users.load()"
    />
    <Section v-else>
      <div v-for="u in usersState.items" :key="u.id" class="urow">
        <div class="top">
          <span class="name">{{ u.full_name }}</span>
          <span class="role" :class="u.role">{{ u.role === 'admin' ? 'Админ' : 'Сотрудник' }}</span>
        </div>
        <div class="sub hint">@{{ u.login }}<span v-if="u.persona"> · {{ u.persona.name }}</span></div>
      </div>
    </Section>
  </div>
</template>

<style scoped>
.title {
  font-size: 22px;
  margin: 16px 0 12px;
}
.urow {
  padding: 12px 14px;
  border-bottom: 1px solid var(--c-secondary-bg);
}
.urow:last-child {
  border-bottom: none;
}
.top {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
}
.name {
  font-weight: 500;
}
.role {
  font-size: 12px;
  padding: 2px 8px;
  border-radius: 999px;
  background: var(--c-secondary-bg);
  color: var(--c-hint);
}
.role.admin {
  background: color-mix(in srgb, var(--c-primary) 15%, var(--c-bg));
  color: var(--c-primary);
}
.sub {
  margin-top: 2px;
}
</style>
