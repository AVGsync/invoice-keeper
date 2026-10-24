<script setup lang="ts">
// the "receipt" on the right side of the login screen, pure decoration
const lines = [
  { date: '18.10', title: 'Аренда офиса', amount: '120 000,00 ₽', note: 'ПРОСРОЧЕНО · 6 ДН.', tone: 'red' },
  { date: '25.10', title: 'Сервер VPS', amount: '49,90 $', note: 'ЗАВТРА', tone: 'soft' },
  { date: '26.10', title: 'Дизайн-сервис', amount: '15,00 €', note: 'ОПЛАЧЕНО 20.10', tone: 'green' },
  { date: '30.10', title: 'Почта Workspace', amount: '36,00 €', note: 'ЧЕРЕЗ 6 ДН.', tone: 'muted' },
  { date: '31.10', title: 'Домен', amount: '1 990,00 ₽', note: 'ЧЕРЕЗ 7 ДН.', tone: 'muted' },
]
</script>

<template>
  <div class="receipt-wrap">
    <div class="receipt mono">
      <p class="head"><span class="strong">НАПОМИНАНИЯ</span><span class="muted">ОКТ 2026</span></p>
      <p class="dashes">- - - - - - - - - - - - - - - - - - - - - - - -</p>

      <div v-for="line in lines" :key="line.date" class="line">
        <span class="date soft">{{ line.date }}</span>
        <span>
          <span class="title">{{ line.title }}</span>
          <span class="note" :class="line.tone">{{ line.note }}</span>
        </span>
        <span class="amount">{{ line.amount }}</span>
      </div>

      <p class="dashes">- - - - - - - - - - - - - - - - - - - - - - - -</p>
      <p class="kicker">Итого за месяц</p>
      <UiLeaderRow><template #label><span class="soft">RUB</span></template><span class="strong">123 490,00 ₽</span></UiLeaderRow>
      <UiLeaderRow><template #label><span class="soft">USD</span></template><span class="strong">61,90 $</span></UiLeaderRow>
      <UiLeaderRow><template #label><span class="soft">EUR</span></template><span class="strong">51,00 €</span></UiLeaderRow>

      <div class="stamp-spot">
        <UiStamp color="accent" :tilt="7.5" large class="stamp">ПРОВЕРЕНО 24.10</UiStamp>
      </div>
    </div>

    <p class="caption italic">
      <span class="dot" />
      Telegram напомнит за три дня до срока —<br>аренда, серверы и подписки не потеряются.
    </p>
  </div>
</template>

<style scoped>
.receipt-wrap {
  width: 380px;
}

.receipt {
  /* swap theme colours for paper ones, everything inside picks them up */
  --surface: var(--paper);
  --ink: var(--paper-ink);
  --ink-2: var(--paper-ink-2);
  --ink-3: var(--paper-ink-3);
  --line-strong: var(--paper-line);
  --accent: var(--paper-accent);
  --accent-ink: var(--paper-accent);
  --red: var(--paper-red);
  --green: var(--paper-green);

  position: relative;
  color: var(--ink);
  /* extra room at the bottom so the stamp fits on the paper */
  padding: 28px 28px 104px;
  background: var(--surface);
  font-size: 13px;
}

/* torn paper edge at the bottom */
.receipt::after {
  content: '';
  position: absolute;
  left: 0;
  right: 0;
  bottom: -7px;
  height: 7px;
  background:
    linear-gradient(135deg, var(--surface) 50%, transparent 50%) 0 0 / 10px 7px repeat-x,
    linear-gradient(225deg, var(--surface) 50%, transparent 50%) 0 0 / 10px 7px repeat-x;
}

.head {
  display: flex;
  justify-content: space-between;
  font-size: 11px;
  letter-spacing: 0.08em;
}

.strong {
  font-weight: 500;
}

.dashes {
  margin: 8px 0 14px;
  overflow: hidden;
  font-size: 11px;
  white-space: nowrap;
  color: var(--line-strong);
}

.line {
  display: grid;
  grid-template-columns: 62px 1fr auto;
  column-gap: 16px;
  height: 52px;
}

.title {
  display: block;
  font-family: var(--sans);
  font-size: 14px;
  font-weight: 500;
}

.note {
  font-size: 10px;
  letter-spacing: 0.04em;
}

.note.red { color: var(--red); }
.note.green { color: var(--green); }
.note.soft { color: var(--ink-2); }
.note.muted { color: var(--ink-3); }

.kicker {
  margin-bottom: 9px;
}

.receipt :deep(.row) {
  margin-bottom: 5px;
}

.stamp-spot {
  position: absolute;
  left: 0;
  right: 0;
  bottom: 34px;
  text-align: center;
}

.stamp {
  font-size: 11px;
  transform-origin: center;
}

.caption {
  display: flex;
  gap: 9px;
  margin-top: 26px;
  font-size: 15px;
  line-height: 21px;
  color: var(--ink-2);
}

.dot {
  flex-shrink: 0;
  width: 7px;
  height: 7px;
  margin-top: 12px;
  border-radius: 50%;
  background: var(--accent);
}
</style>
