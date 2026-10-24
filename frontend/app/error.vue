<!-- shown by nuxt for unknown pages (404) and crashes -->
<script setup lang="ts">
import type { NuxtError } from '#app'

const props = defineProps<{ error: NuxtError }>()

const { user, home } = useAuth()
const notFound = computed(() => props.error.statusCode === 404)

function goHome() {
  clearError({ redirect: user.value ? home.value : '/login' })
}
</script>

<template>
  <main class="error-page">
    <AppLogo />

    <div class="body">
      <p class="kicker">{{ notFound ? 'Страница не найдена' : 'Ошибка' }} · {{ error.statusCode }}</p>
      <p class="code display">{{ error.statusCode }}</p>
      <h1 class="title display">{{ notFound ? 'Такой строки в ведомости нет' : 'Что-то сломалось' }}</h1>
      <p class="text">
        {{ notFound
          ? 'Ссылка устарела или в адресе опечатка. Все платежи на месте.'
          : 'Попробуйте обновить страницу. Если не поможет — вернитесь на главную.' }}
      </p>
      <p class="dashes mono" aria-hidden="true">- - - - - - - - - - - - - - - - - - - - -</p>
      <UiButton variant="primary" size="lg" @click="goHome">На главную</UiButton>
    </div>

    <AppMascot class="mascot" :pose="notFound ? 'lost' : 'fragile'" :size="220" />

    <p class="folio kicker">Invoice Keeper · ведомость платежей</p>
  </main>
</template>

<style scoped>
.error-page {
  position: relative;
  display: flex;
  flex-direction: column;
  min-height: 100vh;
  padding: 40px 48px 32px;
}

.body {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 12px;
  width: 460px;
  margin: auto 0 auto 152px;
}

.mascot {
  position: absolute;
  top: 50%;
  right: 12vw;
  translate: 0 -50%;
}

.code {
  font-size: 96px;
  line-height: 1;
  letter-spacing: -0.04em;
  color: var(--ink-3);
}

.title {
  font-size: 24px;
  line-height: 32px;
}

.text {
  font-size: 15px;
  line-height: 22px;
  color: var(--ink-2);
}

.dashes {
  overflow: hidden;
  font-size: 11px;
  white-space: nowrap;
  color: var(--line-strong);
}
</style>
