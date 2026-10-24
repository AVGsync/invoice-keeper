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
      <component
        :is="u.role === 'admin' ? 'div' : 'button'"
        v-for="u in usersState.items"
        :key="u.id"
        class="urow"
        :class="{ clickable: u.role !== 'admin' }"
        :type="u.role === 'admin' ? undefined : 'button'"
        @click="u.role !== 'admin' && router.push('/admin/users/' + u.id)"
      >
        <div class="top">
          <span class="name">{{ u.full_name }}</span>
          <span class="role" :class="u.role">{{ u.role === 'admin' ? 'Админ' : 'Сотрудник' }}</span>
        </div>
        <div class="sub hint">@{{ u.login }}<span v-if="u.persona"> · {{ u.persona.name }}</span></div>
        <div class="sub hint tg" :class="{ on: u.telegram_linked }">
          {{
            u.telegram_linked
              ? 'Telegram привязан' + (u.telegram_username ? ' · @' + u.telegram_username : '')
              : 'Telegram не привязан'
          }}
        </div>
        <div class="sub hint tg" :class="{ on: u.email_verified }">
          {{ u.email_verified ? 'Почта подтверждена' : u.email_linked ? 'Почта не подтверждена' : 'Почта не привязана' }}
        </div>
      </component>
    </Section>
  </div>
</template>

<style scoped>
.urow.clickable {
  display: block;
  width: 100%;
  text-align: left;
  border: 0;
  border-bottom: 1px solid var(--c-line);
  background: transparent;
  color: inherit;
  font: inherit;
}
.urow.clickable:active {
  opacity: 0.55;
}
.urow {
  padding: var(--sp-4) 0;
  border-bottom: 1px solid var(--c-line);
}
.top {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
}
.name {
  font-size: var(--fs-body-l);
  font-weight: 400;
}
.role {
  font-size: 12px;
  font-weight: 500;
  padding-left: 0;
  color: var(--fog);
  border: 1px solid transparent;
  border-radius: var(--radius-pill);
}
.role.admin {
  padding: 0 8px;
  height: 22px;
  display: inline-flex;
  align-items: center;
  border-color: var(--snow);
  background: var(--snow);
  color: var(--ink);
}
.tg.on {
  color: var(--ash-text);
}
.sub {
  font-size: var(--fs-small);
  color: var(--fog);
  margin-top: 4px;
}
</style>
