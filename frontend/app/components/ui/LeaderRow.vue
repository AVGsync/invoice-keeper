<!-- label on the left, value on the right -->
<template>
  <div class="row">
    <span class="label"><slot name="label" /></span>
    <span class="leader" />
    <span class="value mono"><slot /></span>
  </div>
</template>

<style scoped>
.row {
  display: flex;
  align-items: baseline;
  gap: 8px;
  font-size: 13px;
}

.label {
  color: var(--ink-2);
}

.leader {
  flex: 1;
}
</style>
