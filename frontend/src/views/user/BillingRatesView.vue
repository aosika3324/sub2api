<template>
  <AppLayout>
    <div class="space-y-6">
      <!-- ============ Cost estimator ============ -->
      <BillingEstimator v-if="!loading && estimatorModels.length > 0" :models="estimatorModels" />

      <!-- ============ Hero / explainer ============ -->
      <section class="card overflow-hidden">
        <div class="flex flex-col gap-4 p-5 xl:flex-row xl:items-start xl:justify-between">
          <div class="min-w-0">
            <div class="flex items-start gap-3">
              <div class="rounded-lg bg-accent-100 p-2 dark:bg-accent-900/30">
                <Icon name="dollar" size="md" class="text-accent-600 dark:text-accent-300" />
              </div>
              <div class="min-w-0">
                <p class="text-xs font-semibold uppercase tracking-wide text-accent-600 dark:text-accent-300">
                  {{ t('billingRates.explainerEyebrow') }}
                </p>
                <h2 class="mt-1 text-lg font-semibold text-[var(--ui-text)]">
                  {{ t('billingRates.explainerTitle') }}
                </h2>
              </div>
            </div>
            <p class="mt-3 max-w-2xl text-sm leading-6 text-[var(--ui-muted)]">
              {{ t('billingRates.explainerDescription') }}
            </p>

            <!-- Best saving highlight -->
            <div v-if="bestSavingPercent > 0" class="mt-4 inline-flex items-center gap-2 rounded-lg bg-emerald-50 px-3 py-2 dark:bg-emerald-900/20">
              <Icon name="sparkles" size="sm" class="text-emerald-600 dark:text-emerald-300" />
              <span class="text-sm font-semibold text-emerald-700 dark:text-emerald-300">
                {{ t('billingRates.yourBestSaving', { percent: bestSavingPercent }) }}
              </span>
            </div>

            <!-- How price works (collapsed) -->
            <details class="mt-4 max-w-2xl rounded-lg border border-[var(--ui-border)]">
              <summary class="cursor-pointer select-none px-4 py-2.5 text-sm font-medium text-[var(--ui-text)]">
                {{ t('billingRates.howPriceWorks') }}
              </summary>
              <p class="border-t border-[var(--ui-border)] px-4 py-3 text-xs leading-5 text-[var(--ui-muted)]">
                {{ t('billingRates.howPriceWorksText') }}
              </p>
            </details>
          </div>

          <button
            @click="loadRates"
            :disabled="loading"
            class="btn btn-secondary w-full shrink-0 xl:w-auto"
          >
            <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
            <span class="ml-2">{{ t('billingRates.refresh') }}</span>
          </button>
        </div>
      </section>

      <!-- ============ Toolbar ============ -->
      <div class="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
        <div class="relative w-full sm:max-w-sm">
          <Icon name="search" size="md" class="absolute left-3 top-1/2 -translate-y-1/2 text-[var(--ui-faint)]" />
          <input
            v-model="searchQuery"
            type="text"
            :placeholder="t('billingRates.searchPlaceholder')"
            class="input pl-10"
          />
        </div>

        <div class="flex flex-wrap items-center gap-3">
          <!-- Platform filter -->
          <div v-if="platforms.length > 1" class="flex flex-wrap items-center gap-1.5">
            <button
              type="button"
              class="rounded-md px-2.5 py-1 text-xs font-medium transition-colors"
              :class="selectedPlatform === ''
                ? 'bg-accent-500 text-white'
                : 'border border-[var(--ui-border)] text-[var(--ui-muted)] hover:bg-[var(--ui-surface-muted)]'"
              @click="selectedPlatform = ''"
            >
              {{ t('billingRates.allPlatforms') }}
            </button>
            <button
              v-for="p in platforms"
              :key="p"
              type="button"
              class="inline-flex items-center gap-1 rounded-md px-2.5 py-1 text-xs font-medium uppercase transition-colors"
              :class="selectedPlatform === p
                ? 'bg-accent-500 text-white'
                : 'border border-[var(--ui-border)] text-[var(--ui-muted)] hover:bg-[var(--ui-surface-muted)]'"
              @click="selectedPlatform = selectedPlatform === p ? '' : p"
            >
              <PlatformIcon :platform="p as GroupPlatform" size="xs" />
              {{ p }}
            </button>
          </div>

          <!-- Sort -->
          <select v-model="sortMode" class="input w-auto" :aria-label="t('billingRates.sortLabel')">
            <option value="cheapest">{{ t('billingRates.sortCheapest') }}</option>
            <option value="name">{{ t('billingRates.sortNameAsc') }}</option>
          </select>

          <RouterLink to="/usage" class="btn btn-secondary justify-center">
            <Icon name="chart" size="md" />
            <span class="ml-2">{{ t('billingRates.viewUsage') }}</span>
          </RouterLink>
        </div>
      </div>

      <!-- ============ Loading skeleton ============ -->
      <div v-if="loading" class="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
        <div v-for="i in 6" :key="i" class="card p-5">
          <div class="h-5 w-32 animate-pulse rounded bg-[var(--ui-surface-muted)]"></div>
          <div class="mt-3 h-4 w-48 animate-pulse rounded bg-[var(--ui-surface-muted)]"></div>
          <div class="mt-4 h-8 w-full animate-pulse rounded bg-[var(--ui-surface-muted)]"></div>
        </div>
      </div>

      <!-- ============ Empty states ============ -->
      <div v-else-if="filteredModelGroups.length === 0" class="card px-5 py-12 text-center">
        <Icon name="inbox" size="xl" class="mx-auto mb-3 text-[var(--ui-faint)]" />
        <p class="text-sm text-[var(--ui-muted)]">
          {{ searchQuery.trim() ? t('billingRates.noMatches', { q: searchQuery.trim() }) : t('billingRates.emptyModels') }}
        </p>
        <button v-if="searchQuery.trim()" class="btn btn-secondary mt-4" @click="searchQuery = ''">
          {{ t('billingRates.clearSearch') }}
        </button>
      </div>

      <!-- ============ Model card grid ============ -->
      <div v-else class="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
        <article
          v-for="modelGroup in filteredModelGroups"
          :key="modelGroup.model"
          class="card relative flex flex-col overflow-hidden"
        >
          <!-- Best value ribbon -->
          <div
            v-if="modelGroup.model === bestValueModel"
            class="absolute right-0 top-0 rounded-bl-lg bg-emerald-500 px-2 py-0.5 text-[10px] font-bold uppercase tracking-wide text-white"
          >
            {{ t('billingRates.bestValue') }}
          </div>

          <!-- Header -->
          <div class="border-b border-[var(--ui-border)] px-5 py-4">
            <div class="flex items-start gap-2 pr-16">
              <h3 class="min-w-0 break-words text-base font-semibold text-[var(--ui-text)]">
                {{ modelGroup.model }}
              </h3>
            </div>
            <div class="mt-2 flex flex-wrap gap-1.5">
              <span
                v-for="platform in modelGroup.platforms"
                :key="platform"
                :class="['inline-flex items-center gap-1 rounded-md border px-2 py-0.5 text-xs font-medium uppercase', platformClass(platform)]"
              >
                <PlatformIcon :platform="platform as GroupPlatform" size="xs" />
                {{ platform }}
              </span>
            </div>
            <p class="mt-2 text-sm text-[var(--ui-muted)]">{{ priceSummary(modelGroup.best.effective_pricing) }}</p>
          </div>

          <!-- Body: your price -->
          <div class="flex-1 px-5 py-4">
            <div class="text-xs font-medium text-[var(--ui-muted)]">{{ t('billingRates.yourPriceLabel') }}</div>
            <div class="mt-2">
              <PricingLines :lines="strongPricingLines(modelGroup.best.effective_pricing)" :empty="t('billingRates.noPricing')" strong />
            </div>

            <!-- Saving badge -->
            <div class="mt-3 flex flex-wrap items-center gap-1.5">
              <span
                v-if="savingPercent(modelGroup.best.applied_multiplier) > 0"
                class="rounded bg-emerald-50 px-2 py-1 text-xs font-semibold text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300"
              >
                {{ t('billingRates.savingBadge', { percent: savingPercent(modelGroup.best.applied_multiplier) }) }}
              </span>
              <span
                v-else-if="savingPercent(modelGroup.best.applied_multiplier) < 0"
                class="rounded bg-amber-50 px-2 py-1 text-xs font-semibold text-amber-700 dark:bg-amber-900/30 dark:text-amber-200"
              >
                {{ t('billingRates.premiumBadge', { percent: -savingPercent(modelGroup.best.applied_multiplier) }) }}
              </span>
              <span v-else class="rounded bg-[var(--ui-surface-muted)] px-2 py-1 text-xs font-medium text-[var(--ui-muted)]">
                {{ t('billingRates.standardBadge') }}
              </span>
            </div>

            <p v-if="modelGroup.rows.length > 1" class="mt-2 text-xs text-[var(--ui-faint)]">
              {{ t('billingRates.moreVariants', { count: modelGroup.rows.length - 1 }) }}
            </p>
          </div>

          <!-- Billing details (progressive disclosure) -->
          <details class="border-t border-[var(--ui-border)]">
            <summary class="cursor-pointer select-none px-5 py-3 text-xs font-medium text-[var(--ui-muted)] hover:text-[var(--ui-text)]">
              {{ t('billingRates.billingDetails') }}
            </summary>
            <div class="space-y-3 border-t border-[var(--ui-border)] px-5 py-4">
              <p class="text-xs text-[var(--ui-faint)]">{{ t('billingRates.whyThisPrice') }}</p>

              <div
                v-for="(row, index) in modelGroup.rows"
                :key="`${row.group.id}-${row.multiplier_type}-${index}`"
                class="rounded-lg border border-[var(--ui-border)] p-3"
              >
                <div class="flex flex-wrap items-center gap-2">
                  <GroupBadge
                    :name="row.group.name"
                    :platform="row.group.platform as GroupPlatform"
                    :subscription-type="(row.group.subscription_type || 'standard') as SubscriptionType"
                    :rate-multiplier="row.group.default_rate_multiplier"
                    :user-rate-multiplier="row.group.custom_rate_multiplier ?? null"
                    always-show-rate
                  />
                  <span
                    v-if="row.multiplier_type === 'image'"
                    class="rounded bg-pink-50 px-2 py-0.5 text-xs font-medium text-pink-700 dark:bg-pink-900/30 dark:text-pink-300"
                  >
                    {{ t('billingRates.advanced.variantImage') }}
                  </span>
                  <span class="text-xs text-[var(--ui-faint)]">
                    {{ t('billingRates.advanced.coefficient') }}: {{ formatRate(row.applied_multiplier) }}
                  </span>
                </div>

                <div class="mt-2 grid gap-3 sm:grid-cols-2">
                  <div>
                    <div class="mb-1 text-xs font-medium text-[var(--ui-muted)]">{{ t('billingRates.columns.billedPrice') }}</div>
                    <PricingLines :lines="pricingLines(row.effective_pricing)" :empty="t('billingRates.noPricing')" strong />
                  </div>
                  <div>
                    <div class="mb-1 text-xs font-medium text-[var(--ui-muted)]">{{ t('billingRates.columns.basePrice') }}</div>
                    <PricingLines :lines="pricingLines(row.base_pricing)" :empty="t('billingRates.noPricing')" struck />
                    <div class="mt-1 text-xs text-[var(--ui-faint)]">{{ pricingSourceLabel(row.pricing_source) }}</div>
                  </div>
                </div>
              </div>
            </div>
          </details>
        </article>
      </div>

      <!-- Footer note -->
      <p v-if="!loading && filteredModelGroups.length > 0" class="px-1 text-xs text-[var(--ui-faint)]">
        {{ t('billingRates.footerNote') }}
      </p>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import GroupBadge from '@/components/common/GroupBadge.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import BillingEstimator, { type EstimatorModel } from '@/components/user/BillingEstimator.vue'
import billingRatesAPI, {
  type UserBillingRateGroup,
  type UserBillingRateModel
} from '@/api/billingRates'
import type { UserSupportedModelPricing } from '@/api/channels'
import type { GroupPlatform, SubscriptionType } from '@/types'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatScaled } from '@/utils/pricing'
import { formatMultiplier } from '@/utils/formatters'
import { platformBadgeClass } from '@/utils/platformColors'
import {
  BILLING_MODE_IMAGE,
  BILLING_MODE_PER_REQUEST,
  BILLING_MODE_TOKEN
} from '@/constants/channel'

const PricingLines = defineComponent({
  name: 'PricingLines',
  props: {
    lines: { type: Array as () => string[], required: true },
    empty: { type: String, required: true },
    strong: { type: Boolean, default: false },
    struck: { type: Boolean, default: false }
  },
  setup(props) {
    return () => {
      if (props.lines.length === 0) {
        return h('span', { class: 'text-[var(--ui-faint)]' }, props.empty)
      }
      return h(
        'div',
        { class: 'space-y-1' },
        props.lines.map((line) =>
          h('div', {
            class: [
              'whitespace-nowrap text-xs',
              props.strong
                ? 'font-semibold text-emerald-700 dark:text-emerald-300'
                : props.struck
                  ? 'text-[var(--ui-faint)] line-through'
                  : 'text-[var(--ui-muted)]'
            ]
          }, line)
        )
      )
    }
  }
})

const { t } = useI18n()
const appStore = useAppStore()

const groups = ref<UserBillingRateGroup[]>([])
const models = ref<UserBillingRateModel[]>([])
const loading = ref(false)
const searchQuery = ref('')
const selectedPlatform = ref('')
const sortMode = ref<'cheapest' | 'name'>('cheapest')
let abortController: AbortController | null = null

interface ModelRateGroup {
  model: string
  rows: UserBillingRateModel[]
  best: UserBillingRateModel
  minMultiplier: number
  platforms: string[]
}

// Distinct platforms across all visible models (for the filter pills).
const platforms = computed(() =>
  Array.from(new Set(models.value.map((m) => m.platform).filter(Boolean))).sort((a, b) =>
    a.localeCompare(b, undefined, { sensitivity: 'base' })
  )
)

// Lowest standard multiplier across accessible groups → headline saving.
const bestSavingPercent = computed(() => {
  const lowest = groups.value.reduce<number | null>((min, g) => {
    if (min == null || g.effective_multiplier < min) return g.effective_multiplier
    return min
  }, null)
  return lowest == null ? 0 : Math.max(0, Math.round((1 - lowest) * 100))
})

const filteredModelGroups = computed<ModelRateGroup[]>(() => {
  const q = searchQuery.value.trim().toLowerCase()
  let rows = models.value
  if (selectedPlatform.value) rows = rows.filter((r) => r.platform === selectedPlatform.value)
  if (q) rows = rows.filter((r) => modelRateMatchesSearch(r, q))

  const byModel = new Map<string, UserBillingRateModel[]>()
  for (const row of rows) {
    const modelName = row.model || '-'
    const bucket = byModel.get(modelName)
    if (bucket) bucket.push(row)
    else byModel.set(modelName, [row])
  }

  const result = Array.from(byModel.entries()).map(([model, modelRows]) => {
    const sortedRows = [...modelRows].sort(compareModelRateRows)
    const best = sortedRows[0]
    return {
      model,
      rows: sortedRows,
      best,
      minMultiplier: best?.applied_multiplier ?? 0,
      platforms: Array.from(new Set(sortedRows.map((r) => r.platform).filter(Boolean))).sort((a, b) =>
        a.localeCompare(b, undefined, { sensitivity: 'base' })
      )
    }
  })

  if (sortMode.value === 'name') {
    result.sort((a, b) => a.model.localeCompare(b.model, undefined, { sensitivity: 'base' }))
  } else {
    // Cheapest first by best effective output price; models without a comparable
    // price sink to the bottom.
    result.sort((a, b) => {
      const ca = comparablePrice(a.best.effective_pricing)
      const cb = comparablePrice(b.best.effective_pricing)
      if (ca == null && cb == null) return a.model.localeCompare(b.model, undefined, { sensitivity: 'base' })
      if (ca == null) return 1
      if (cb == null) return -1
      return ca - cb
    })
  }
  return result
})

// The cheapest model overall (by comparable price) → "best value" ribbon.
const bestValueModel = computed(() => {
  let best: { model: string; price: number } | null = null
  for (const g of filteredModelGroups.value) {
    const price = comparablePrice(g.best.effective_pricing)
    if (price == null) continue
    if (!best || price < best.price) best = { model: g.model, price }
  }
  return best?.model ?? ''
})

// One estimable entry per model (its best/cheapest variant) for the estimator.
// Built from the FULL model list — not the filtered grid — so the estimator
// stays a standalone "what will I spend" tool that search/platform filters
// never shrink or hide.
const estimatorModels = computed<EstimatorModel[]>(() => {
  const byModel = new Map<string, UserBillingRateModel>()
  for (const row of models.value) {
    const name = row.model || '-'
    const current = byModel.get(name)
    if (!current || row.applied_multiplier < current.applied_multiplier) {
      byModel.set(name, row)
    }
  }
  return Array.from(byModel.entries())
    .map(([model, row]) => ({ model, pricing: row.effective_pricing }))
    .sort((a, b) => a.model.localeCompare(b.model, undefined, { sensitivity: 'base' }))
})

async function loadRates() {
  if (abortController) abortController.abort()
  const current = new AbortController()
  abortController = current
  loading.value = true
  try {
    const data = await billingRatesAPI.getBillingRates({ signal: current.signal })
    if (current.signal.aborted) return
    groups.value = data.groups
    models.value = data.models
  } catch (err: unknown) {
    if (current.signal.aborted) return
    appStore.showError(extractApiErrorMessage(err, t('billingRates.loadFailed')))
  } finally {
    if (abortController === current) loading.value = false
  }
}

function formatRate(value: number): string {
  return `${formatMultiplier(value)}x`
}

// round((1 - multiplier) * 100): positive = discount, negative = premium.
function savingPercent(multiplier: number): number {
  return Math.round((1 - multiplier) * 100)
}

// A single comparable USD figure for sorting (prefers output token price).
// A single comparable cost yardstick across billing modes: the cost of one
// representative request (1k input + 3k output tokens for token models, one
// charge for per-request/image models). This makes token/per_request/image
// models rankable on the same scale and keeps the "best value" ribbon and the
// estimator's "cheapest" in agreement (both use the same assumptions).
const COMPARE_INPUT_TOKENS = 1000
const COMPARE_OUTPUT_TOKENS = 3000

function comparablePrice(pricing: UserSupportedModelPricing | null): number | null {
  if (!pricing) return null
  if (pricing.billing_mode === BILLING_MODE_TOKEN) {
    if (pricing.input_price == null && pricing.output_price == null) return null
    return COMPARE_INPUT_TOKENS * (pricing.input_price ?? 0) + COMPARE_OUTPUT_TOKENS * (pricing.output_price ?? 0)
  }
  if (pricing.billing_mode === BILLING_MODE_PER_REQUEST) return pricing.per_request_price ?? null
  if (pricing.billing_mode === BILLING_MODE_IMAGE) {
    return pricing.image_output_price ?? pricing.per_request_price ?? null
  }
  return null
}

function platformClass(platform: string): string {
  if (!platform) {
    return 'border-[var(--ui-border)] bg-[var(--ui-surface-muted)] text-[var(--ui-muted)]'
  }
  return platformBadgeClass(platform)
}

function modelRateMatchesSearch(row: UserBillingRateModel, query: string): boolean {
  return [row.model, row.platform].filter(Boolean).some((v) => String(v).toLowerCase().includes(query))
}

function compareModelRateRows(a: UserBillingRateModel, b: UserBillingRateModel): number {
  const diff = a.applied_multiplier - b.applied_multiplier
  if (diff !== 0) return diff
  return `${a.group.name}`.localeCompare(`${b.group.name}`, undefined, { sensitivity: 'base' })
}

// One-line price summary for a card header.
function priceSummary(pricing: UserSupportedModelPricing | null): string {
  if (!pricing) return t('billingRates.noPricing')
  if (pricing.billing_mode === BILLING_MODE_TOKEN) {
    return t('billingRates.priceSummaryToken', {
      input: formatScaled(pricing.input_price, 1_000_000),
      output: formatScaled(pricing.output_price, 1_000_000)
    })
  }
  if (pricing.billing_mode === BILLING_MODE_IMAGE) {
    return t('billingRates.priceSummaryImage', {
      price: formatScaled(pricing.image_output_price ?? pricing.per_request_price, 1)
    })
  }
  if (pricing.billing_mode === BILLING_MODE_PER_REQUEST) {
    return t('billingRates.priceSummaryPerRequest', { price: formatScaled(pricing.per_request_price, 1) })
  }
  return ''
}

// The two headline prices (input/output) for token models; full set for others.
function strongPricingLines(pricing: UserSupportedModelPricing | null): string[] {
  if (!pricing) return []
  if (pricing.billing_mode === BILLING_MODE_TOKEN) {
    const lines: string[] = []
    pushPrice(lines, t('billingRates.input'), pricing.input_price, 1_000_000, t('billingRates.perMillion'))
    pushPrice(lines, t('billingRates.output'), pricing.output_price, 1_000_000, t('billingRates.perMillion'))
    return lines
  }
  return pricingLines(pricing)
}

function pricingSourceLabel(source: string): string {
  switch (source) {
    case 'channel': return t('billingRates.pricingSourceChannel')
    case 'litellm': return t('billingRates.pricingSourceLiteLLM')
    case 'fallback': return t('billingRates.pricingSourceFallback')
    case 'display': return t('billingRates.pricingSourceDisplay')
    default: return source || t('billingRates.pricingSourceUnknown')
  }
}

function pricingLines(pricing: UserSupportedModelPricing | null): string[] {
  if (!pricing) return []
  const lines: string[] = []
  const perMillion = t('billingRates.perMillion')
  const perRequest = t('billingRates.perRequestUnit')

  if (pricing.billing_mode === BILLING_MODE_TOKEN) {
    pushPrice(lines, t('billingRates.input'), pricing.input_price, 1_000_000, perMillion)
    pushPrice(lines, t('billingRates.output'), pricing.output_price, 1_000_000, perMillion)
    pushPrice(lines, t('billingRates.cacheWrite'), pricing.cache_write_price, 1_000_000, perMillion)
    pushPrice(lines, t('billingRates.cacheRead'), pricing.cache_read_price, 1_000_000, perMillion)
    pushPrice(lines, t('billingRates.imageOutput'), pricing.image_output_price, 1_000_000, perMillion)
  } else if (pricing.billing_mode === BILLING_MODE_PER_REQUEST) {
    pushPrice(lines, t('billingRates.perRequest'), pricing.per_request_price, 1, perRequest)
  } else if (pricing.billing_mode === BILLING_MODE_IMAGE) {
    pushPrice(lines, t('billingRates.imageOutput'), pricing.image_output_price, 1, perRequest)
    if (pricing.image_output_price == null) {
      pushPrice(lines, t('billingRates.perRequest'), pricing.per_request_price, 1, perRequest)
    }
  }

  if (pricing.intervals?.length) {
    lines.push(t('billingRates.intervalCount', { count: pricing.intervals.length }))
  }
  return lines
}

function pushPrice(lines: string[], label: string, value: number | null, scale: number, unit: string) {
  if (value == null) return
  lines.push(`${label}: ${formatScaled(value, scale)} ${unit}`)
}

onMounted(loadRates)
onBeforeUnmount(() => abortController?.abort())
</script>
