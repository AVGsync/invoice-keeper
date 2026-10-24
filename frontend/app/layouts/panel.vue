<template>
  <div class="panel">
    <AppSidebar />
    <main class="main">
      <slot />
      <AppFolio />
    </main>
  </div>
</template>

<style scoped>
.panel {
  display: flex;
  min-height: 100vh;
}

.main {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-width: 0;
  padding: 0 32px 0 31px;
}
</style>
