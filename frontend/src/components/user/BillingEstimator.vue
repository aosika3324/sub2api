<template>
  <section class="card overflow-hidden">
    <div class="border-b border-[var(--ui-border)] px-5 py-4">
      <div class="flex items-center gap-3">
        <div class="rounded-lg bg-accent-100 p-2 dark:bg-accent-900/30">
          <Icon name="calculator" size="md" class="text-accent-600 dark:text-accent-300" />
        </div>
        <div>
          <h3 class="text-base font-semibold text-[var(--ui-text)]">{{ t('billingRates.estimator.title') }}</h3>
          <p class="mt-0.5 text-xs text-[var(--ui-muted)]">{{ t('billingRates.estimator.hint') }}</p>
        </div>
      </div>
    </div>

    <div v-if="estimableModels.length === 0" class="px-5 py-6 text-sm text-[var(--ui-muted)]">
      {{ t('billingRates.estimator.disclaimer') }}
    </div>

    <div v-else class="grid gap-5 px-5 py-5 lg:grid-cols-[1.1fr_0.9fr]">
      <!-- Controls -->
      <div class="space-y-4">
        <!-- Usage presets -->
        <div class="inline-flex w-full rounded-lg border border-[var(--ui-border)] p-1">
          <button
            v-for="preset in presets"
            :key="preset.key"
            type="button"
            class="flex-1 rounded-md px-2 py-1.5 text-xs font-medium transition-colors"
            :class="activePreset === preset.key
              ? 'bg-accent-500 text-white'
              : 'text-[var(--ui-muted)] hover:bg-[var(--ui-surface-muted)]'"
            @click="applyPreset(preset.key)"
          >
            {{ t(`billingRates.estimator.${preset.key}`) }}
          </button>
        </div>

        <!-- Requests/day slider -->
        <div>
          <label class="mb-2 block text-xs font-medium text-[var(--ui-muted)]">
            {{ t('billingRates.estimator.requestsPerDay', { count: requestsPerDay }) }}
          </label>
          <input
            v-model.number="requestsPerDay"
            type="range"
            min="1"
            max="300"
            step="1"
            class="estimator-slider w-full"
          />
        </div>

        <!-- Model picker -->
        <div>
          <label class="mb-2 block text-xs font-medium text-[var(--ui-muted)]">
            {{ t('billingRates.estimator.pickModel') }}
          </label>
          <select v-model="selectedModel" class="input w-full">
            <option v-for="m in estimableModels" :key="m.model" :value="m.model">
              {{ m.model }}
            </option>
          </select>
        </div>
      </div>

      <!-- Result -->
      <div class="flex flex-col justify-center rounded-lg border border-[var(--ui-border)] bg-[var(--ui-surface-muted)] p-5">
        <div class="text-xs font-medium text-[var(--ui-muted)]">
          {{ t('billingRates.estimator.resultPrefix') }}
        </div>
        <div class="mt-1 flex items-baseline gap-1">
          <span class="text-3xl font-bold text-accent-600 dark:text-accent-300">{{ selectedMonthlyCost }}</span>
          <span class="text-sm text-[var(--ui-muted)]">{{ t('billingRates.estimator.resultSuffix') }}</span>
        </div>

        <div
          v-if="cheapest && cheapest.model !== selectedModel"
          class="mt-3 rounded-md bg-emerald-50 px-3 py-2 text-xs text-emerald-700 dark:bg-emerald-900/20 dark:text-emerald-300"
        >
          {{ t('billingRates.estimator.cheapestHint', { model: cheapest.model, price: cheapestMonthlyCost }) }}
        </div>

        <p class="mt-3 flex items-start gap-1.5 text-[11px] leading-4 text-[var(--ui-faint)]">
          <Icon name="infoCircle" size="xs" class="mt-px flex-shrink-0" />
          <span>{{ t('billingRates.estimator.disclaimer') }}</span>
        </p>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import type { UserSupportedModelPricing } from '@/api/channels'
import {
  BILLING_MODE_IMAGE,
  BILLING_MODE_PER_REQUEST,
  BILLING_MODE_TOKEN
} from '@/constants/channel'

export interface EstimatorModel {
  model: string
  pricing: UserSupportedModelPricing | null
}

const props = defineProps<{
  models: EstimatorModel[]
}>()

const { t } = useI18n()

// A typical assistant request, fixed at a 1:3 input:output ratio (hidden from the
// user). These are deliberately round numbers — the estimator is a rough guide.
const AVG_INPUT_TOKENS = 1000
const AVG_OUTPUT_TOKENS = 3000
const DAYS_PER_MONTH = 30

const presets = [
  { key: 'light', requests: 5 },
  { key: 'medium', requests: 30 },
  { key: 'heavy', requests: 150 }
] as const

const requestsPerDay = ref(30)
const selectedModel = ref<string>('')

// Only models with usable pricing can be estimated.
const estimableModels = computed(() =>
  props.models.filter((m) => m.pricing != null && monthlyCostFor(m.pricing) != null)
)

const activePreset = computed(() => {
  const match = presets.find((p) => p.requests === requestsPerDay.value)
  return match?.key ?? ''
})

function applyPreset(key: string) {
  const preset = presets.find((p) => p.key === key)
  if (preset) requestsPerDay.value = preset.requests
}

// Per-model monthly cost in USD, or null when pricing cannot be estimated.
function monthlyCostFor(pricing: UserSupportedModelPricing | null): number | null {
  if (!pricing) return null
  const daily = requestsPerDay.value
  if (pricing.billing_mode === BILLING_MODE_TOKEN) {
    if (pricing.input_price == null && pricing.output_price == null) return null
    const perRequest =
      AVG_INPUT_TOKENS * (pricing.input_price ?? 0) +
      AVG_OUTPUT_TOKENS * (pricing.output_price ?? 0)
    return perRequest * daily * DAYS_PER_MONTH
  }
  if (pricing.billing_mode === BILLING_MODE_PER_REQUEST) {
    if (pricing.per_request_price == null) return null
    return pricing.per_request_price * daily * DAYS_PER_MONTH
  }
  if (pricing.billing_mode === BILLING_MODE_IMAGE) {
    const unit = pricing.image_output_price ?? pricing.per_request_price
    if (unit == null) return null
    return unit * daily * DAYS_PER_MONTH
  }
  return null
}

function modelCost(model: string): number | null {
  const found = estimableModels.value.find((m) => m.model === model)
  return found ? monthlyCostFor(found.pricing) : null
}

const cheapest = computed(() => {
  let best: { model: string; cost: number } | null = null
  for (const m of estimableModels.value) {
    const cost = monthlyCostFor(m.pricing)
    if (cost == null) continue
    if (!best || cost < best.cost) best = { model: m.model, cost }
  }
  return best
})

const selectedMonthlyCost = computed(() => formatMoney(modelCost(selectedModel.value)))
const cheapestMonthlyCost = computed(() => formatMoney(cheapest.value?.cost ?? null))

function formatMoney(value: number | null): string {
  if (value == null) return '-'
  if (value > 0 && value < 0.01) return '<$0.01'
  if (value >= 100) return `$${value.toFixed(0)}`
  return `$${value.toFixed(2)}`
}

// Default the picker to the cheapest model, and keep it valid as the list changes.
watch(
  estimableModels,
  (list) => {
    if (list.length === 0) {
      selectedModel.value = ''
      return
    }
    if (!list.some((m) => m.model === selectedModel.value)) {
      selectedModel.value = cheapest.value?.model ?? list[0].model
    }
  },
  { immediate: true }
)
</script>

<style scoped>
.estimator-slider {
  -webkit-appearance: none;
  appearance: none;
  height: 6px;
  border-radius: 9999px;
  background: var(--ui-border);
  outline: none;
}

.estimator-slider::-webkit-slider-thumb {
  -webkit-appearance: none;
  appearance: none;
  width: 18px;
  height: 18px;
  border-radius: 9999px;
  background: var(--ui-accent);
  cursor: pointer;
  border: 2px solid var(--ui-surface);
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.2);
}

.estimator-slider::-moz-range-thumb {
  width: 18px;
  height: 18px;
  border-radius: 9999px;
  background: var(--ui-accent);
  cursor: pointer;
  border: 2px solid var(--ui-surface);
}
</style>
