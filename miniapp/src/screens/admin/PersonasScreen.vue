<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { users, usersState } from '../../stores/users'
import { useMainButton } from '../../composables/useMainButton'
import Section from '../../components/Section.vue'
import Cell from '../../components/Cell.vue'
import Skeleton from '../../components/Skeleton.vue'
import EmptyState from '../../components/EmptyState.vue'

const router = useRouter()
onMounted(() => void users.loadPersonas())
const firstLoad = computed(() => !usersState.personasLoaded)

function count(n: number): string {
  const m = n % 100
  const w = m >= 11 && m <= 14 ? 'сотрудников' : n % 10 === 1 ? 'сотрудник' : n % 10 >= 2 && n % 10 <= 4 ? 'сотрудника' : 'сотрудников'
  return `${n} ${w}`
}

useMainButton({ text: 'Новая персона', onClick: () => void router.push('/admin/personas/new') })
</script>

<template>
  <div class="personas">
    <h1 class="title">Персоны</h1>
    <Skeleton v-if="firstLoad" :rows="4" />
    <EmptyState v-else-if="!usersState.personas.length" text="Персон пока нет" />
    <Section v-else>
      <Cell
        v-for="p in usersState.personas"
        :key="p.id"
        clickable
        chevron
        @click="router.push('/admin/personas/' + p.id)"
      >
        <div class="name">{{ p.name }}</div>
        <div class="hint sub">{{ count(p.users_count) }}<span v-if="p.description"> · {{ p.description }}</span></div>
      </Cell>
    </Section>
  </div>
</template>

<style scoped>
.name {
  font-weight: 500;
}
.sub {
  font-size: var(--fs-small);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
