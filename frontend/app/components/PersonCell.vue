<script setup lang="ts">
import type { Person } from '~/utils/periods'

const props = defineProps<{ person: Person }>()
const { isAdmin } = useRole()
const { meName } = usePayments()

// on user screens your own payments get a blue avatar and "· вы"
const me = computed(() => !isAdmin.value && !!meName.value && props.person.name === meName.value)
</script>

<template>
  <span class="person">
    <UiAvatar :initials="person.initials" :me="me" />
    {{ person.short }}<template v-if="me"> · вы</template>
  </span>
</template>

<style scoped>
.person {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  white-space: nowrap;
}
</style>
