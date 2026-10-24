<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { occurrences } from '../../stores/occurrences'
import { editOccurrence, getOccurrence } from '../../api/endpoints'
import { parseMoneyInput } from '../../lib/money'
import { humanError } from '../../lib/humanError'
import { haptic } from '../../telegram/webapp'
import { useMainButton } from '../../composables/useMainButton'
import Segmented from '../../components/Segmented.vue'
import Skeleton from '../../components/Skeleton.vue'
import type { Currency, PaymentResponse } from '../../api/types'

const route = useRoute()
const router = useRouter()

const occ = ref<PaymentResponse | null>(null)
const description = ref('')
const amountInput = ref('')
const currency = ref<Currency>('RUB')
const submitting = ref(false)
const formError = ref('')

function fill(p: PaymentResponse): void {
  occ.value = p
  description.value = p.description
  amountInput.value = (p.amount / 100).toFixed(2).replace('.', ',')
  currency.value = p.currency
}

watch(
  () => route.params.id,
  async (id) => {
    if (typeof id !== 'string') return
    const cached = occurrences.byId(id)
    if (cached) return fill(cached)
    try {
      fill(await getOccurrence(id))
    } catch {
      formError.value = 'Платёж не найден'
    }
  },
  { immediate: true },
)

const amountMinor = computed(() => parseMoneyInput(amountInput.value))
const descTrim = computed(() => description.value.trim())
const changed = computed(
  () =>
    !!occ.value &&
    (descTrim.value !== occ.value.description ||
      amountMinor.value !== occ.value.amount ||
      currency.value !== occ.value.currency),
)
const valid = computed(
  () => descTrim.value.length >= 1 && descTrim.value.length <= 255 && amountMinor.value !== null && changed.value,
)

async function submit(): Promise<void> {
  if (!occ.value || !valid.value || submitting.value) return
  formError.value = ''
  submitting.value = true
  try {
    const body: { amount?: number; currency?: Currency; description?: string } = {}
    if (descTrim.value !== occ.value.description) body.description = descTrim.value
    if (amountMinor.value !== occ.value.amount) body.amount = amountMinor.value!
    if (currency.value !== occ.value.currency) body.currency = currency.value
    await editOccurrence(occ.value.id, body)
    await occurrences.refreshOne(occ.value.id)
    haptic.success()
    router.back()
  } catch (e) {
    haptic.error()
    formError.value = humanError(e)
  } finally {
    submitting.value = false
  }
}

useMainButton({ text: 'Сохранить', enabled: valid, loading: submitting, onClick: submit })
</script>

<template>
  <div class="edit">
    <h1 class="title">Изменить платёж</h1>
    <Skeleton v-if="!occ && !formError" :rows="3" />
    <template v-else-if="occ">
      <label class="field">
        <span class="lbl">Название</span>
        <input v-model="description" type="text" maxlength="255" />
      </label>
      <label class="field">
        <span class="lbl">Сумма</span>
        <input v-model="amountInput" type="text" inputmode="decimal" class="num" placeholder="0,00" />
        <span v-if="amountInput && amountMinor === null" class="err">Введите сумму больше нуля</span>
      </label>
      <div class="field">
        <span class="lbl">Валюта</span>
        <Segmented
          :model-value="currency"
          :options="[
            { value: 'RUB', label: 'RUB' },
            { value: 'USD', label: 'USD' },
            { value: 'EUR', label: 'EUR' },
          ]"
          @update:model-value="(v) => (currency = v)"
        />
      </div>
      <p class="hint">Меняется только этот период. Чтобы изменить все будущие, правьте регулярный платёж.</p>
    </template>
    <p v-if="formError" class="form-error">{{ formError }}</p>
  </div>
</template>
