<template>
  <!-- search by phrase isn't built yet -->
  <label class="ask soon" title="Скоро: поиск по фразе — «что просрочено?», «мои платежи», «в долларах»">
    <input class="input" placeholder="Спросить: «что просрочено?»" disabled>
  </label>
</template>

<style scoped>
.ask {
  display: flex;
  align-items: center;
  width: 300px;
  height: 34px;
  padding: 0 12px;
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: 8px;
}

.input {
  flex: 1;
  border: 0;
  outline: none;
  background: none;
  font-size: 13px;
}

.input::placeholder {
  color: var(--ink-3);
}
</style>
