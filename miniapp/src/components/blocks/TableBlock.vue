<script setup lang="ts">
import type { TableBlock } from '../../api/types'

defineProps<{ block: TableBlock }>()

function cell(v: string | null | undefined): string {
  return v == null || v === '' ? '—' : v
}
</script>

<template>
  <div class="tbl-card">
    <div v-if="block.title" class="tbl-title">{{ block.title }}</div>
    <div class="tbl-scroll">
      <table class="tbl">
        <thead>
          <tr>
            <th
              v-for="c in block.columns"
              :key="c.key"
              :class="{ end: c.align === 'end' }"
            >
              {{ c.label }}
            </th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(row, ri) in block.rows" :key="ri">
            <td
              v-for="c in block.columns"
              :key="c.key"
              :class="{ end: c.align === 'end' }"
            >
              {{ cell(row[c.key]) }}
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    <div v-if="block.truncated" class="tbl-more">Показаны не все строки</div>
  </div>
</template>

<style scoped>
.tbl-card {
  background: var(--graphite);
  border: 1px solid var(--line);
  border-radius: var(--radius-ui);
  padding: var(--sp-4);
}
.tbl-title {
  font-size: var(--fs-small);
  font-weight: 500;
  color: var(--ash-text);
  margin-bottom: var(--sp-3);
}
/* горизонтальный скролл с хвостовым спейсером (приём A.1) */
.tbl-scroll {
  overflow-x: auto;
  margin: 0 calc(-1 * var(--sp-4));
  padding: 0 0 0 var(--sp-4);
  scroll-padding-inline: var(--sp-4);
  scrollbar-width: none;
  overscroll-behavior-x: contain;
}
.tbl-scroll::-webkit-scrollbar {
  display: none;
}
.tbl {
  border-collapse: collapse;
  width: max-content;
  min-width: 100%;
}
/* спейсер справа внутри скроллера */
.tbl::after {
  content: '';
  display: table-cell;
  min-width: var(--sp-4);
}
th,
td {
  text-align: left;
  padding: 6px 12px 6px 0;
  font-size: var(--fs-small);
  white-space: nowrap;
  border-bottom: 1px solid var(--line);
}
th {
  color: var(--muted);
  font-weight: 500;
}
td {
  color: var(--bone-text);
}
tbody tr:last-child td {
  border-bottom: 0;
}
.end {
  text-align: right;
  font-variant-numeric: tabular-nums;
}
.tbl-more {
  margin-top: var(--sp-3);
  font-size: var(--fs-caption);
  color: var(--muted);
}
</style>
