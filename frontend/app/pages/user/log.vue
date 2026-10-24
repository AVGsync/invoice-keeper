<script setup lang="ts">
definePageMeta({ layout: 'panel' })
</script>

<!-- a user sees only their own actions -->
<template>
  <div>
    <AppPageHeader title="Журнал" />
    <ScreenLog mine />
  </div>
</template>
