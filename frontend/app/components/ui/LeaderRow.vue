<!-- "label ........ value" line -->
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

.leader {
  flex: 1;
  border-bottom: 1px dotted var(--line-strong);
}
</style>
