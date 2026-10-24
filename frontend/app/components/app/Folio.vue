<script setup lang="ts">
const { isDemo, loadedAt } = usePayments()

const updated = computed(() => {
  if (isDemo.value) return 'Демо-данные'
  if (!loadedAt.value) return ''
  const time = loadedAt.value.toLocaleString('ru-RU', { timeZone: 'Europe/Moscow', day: '2-digit', month: '2-digit', year: 'numeric', hour: '2-digit', minute: '2-digit' })
  return `Обновлено ${time.replace(',', '')} МСК`
})
</script>

<template>
  <footer class="folio kicker">
    <span>Invoice Keeper · Ведомость платежей</span>
    <span>{{ updated }}</span>
  </footer>
</template>

<style scoped>
.folio {
  display: flex;
  justify-content: space-between;
  margin-top: auto;
  padding: 32px 0 16px;
  font-weight: 400;
}
</style>
