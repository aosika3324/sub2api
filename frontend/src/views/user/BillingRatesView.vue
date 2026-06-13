<template>
  <AppLayout>
    <div class="space-y-6">
      <section class="card overflow-hidden">
        <div class="grid gap-0 lg:grid-cols-[1.15fr_0.85fr]">
          <div class="border-b border-gray-100 p-5 dark:border-dark-700 lg:border-b-0 lg:border-r">
            <div class="flex flex-col gap-4 xl:flex-row xl:items-start xl:justify-between">
              <div class="min-w-0">
                <div class="flex items-start gap-3">
                  <div class="rounded-lg bg-amber-100 p-2 dark:bg-amber-900/30">
                    <Icon name="calculator" size="md" class="text-amber-600 dark:text-amber-300" />
                  </div>
                  <div class="min-w-0">
                    <p class="text-xs font-semibold uppercase tracking-wide text-amber-600 dark:text-amber-300">
                      {{ t('billingRates.explainerEyebrow') }}
                    </p>
                    <h2 class="mt-1 text-lg font-semibold text-gray-900 dark:text-white">
                      {{ t('billingRates.explainerTitle') }}
                    </h2>
                  </div>
                </div>
                <p class="mt-3 max-w-3xl text-sm leading-6 text-gray-500 dark:text-gray-400">
                  {{ t('billingRates.explainerDescription') }}
                </p>
              </div>
              <button
                @click="loadRates"
                :disabled="loading"
                class="btn btn-secondary w-full shrink-0 xl:w-auto"
                :title="t('common.refresh')"
              >
                <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
                <span class="ml-2">{{ t('billingRates.refresh') }}</span>
              </button>
            </div>

            <div class="mt-5 rounded-lg border border-gray-100 bg-gray-50/70 p-4 dark:border-dark-700 dark:bg-dark-800/50">
              <p class="text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">
                {{ t('billingRates.formulaTitle') }}
              </p>
              <div class="mt-3 grid gap-3 md:grid-cols-[1fr_auto_1fr_auto_1fr] md:items-center">
                <div>
                  <div class="text-sm font-semibold text-gray-900 dark:text-white">
                    {{ t('billingRates.formulaBase') }}
                  </div>
                  <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                    {{ t('billingRates.formulaBaseHint') }}
                  </p>
                </div>
                <span class="hidden text-sm font-semibold text-gray-400 dark:text-gray-500 md:block">×</span>
                <div>
                  <div class="text-sm font-semibold text-emerald-700 dark:text-emerald-300">
                    {{ t('billingRates.formulaMultiplier') }}
                  </div>
                  <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                    {{ t('billingRates.formulaMultiplierHint') }}
                  </p>
                </div>
                <Icon name="arrowRight" size="sm" class="hidden text-gray-400 dark:text-gray-500 md:block" />
                <div>
                  <div class="text-sm font-semibold text-gray-900 dark:text-white">
                    {{ t('billingRates.formulaResult') }}
                  </div>
                  <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                    {{ t('billingRates.formulaResultHint') }}
                  </p>
                </div>
              </div>
            </div>

            <div class="mt-4 grid gap-3 md:grid-cols-3">
              <div class="flex gap-3 rounded-lg border border-gray-100 p-3 dark:border-dark-700">
                <Icon name="badge" size="md" class="mt-0.5 shrink-0 text-violet-500 dark:text-violet-300" />
                <div>
                  <div class="text-sm font-semibold text-gray-900 dark:text-white">
                    {{ t('billingRates.rulePriorityTitle') }}
                  </div>
                  <p class="mt-1 text-xs leading-5 text-gray-500 dark:text-gray-400">
                    {{ t('billingRates.rulePriorityText') }}
                  </p>
                </div>
              </div>
              <div class="flex gap-3 rounded-lg border border-gray-100 p-3 dark:border-dark-700">
                <Icon name="sparkles" size="md" class="mt-0.5 shrink-0 text-pink-500 dark:text-pink-300" />
                <div>
                  <div class="text-sm font-semibold text-gray-900 dark:text-white">
                    {{ t('billingRates.ruleImageTitle') }}
                  </div>
                  <p class="mt-1 text-xs leading-5 text-gray-500 dark:text-gray-400">
                    {{ t('billingRates.ruleImageText') }}
                  </p>
                </div>
              </div>
              <div class="flex gap-3 rounded-lg border border-gray-100 p-3 dark:border-dark-700">
                <Icon name="clock" size="md" class="mt-0.5 shrink-0 text-sky-500 dark:text-sky-300" />
                <div>
                  <div class="text-sm font-semibold text-gray-900 dark:text-white">
                    {{ t('billingRates.ruleSnapshotTitle') }}
                  </div>
                  <p class="mt-1 text-xs leading-5 text-gray-500 dark:text-gray-400">
                    {{ t('billingRates.ruleSnapshotText') }}
                  </p>
                </div>
              </div>
            </div>
          </div>

          <aside class="p-5">
            <div class="flex items-center justify-between gap-3">
              <div>
                <h3 class="text-base font-semibold text-gray-900 dark:text-white">
                  {{ t('billingRates.currentAccess') }}
                </h3>
                <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                  {{ t('billingRates.currentAccessHint') }}
                </p>
              </div>
              <span class="rounded bg-emerald-50 px-2 py-1 text-xs font-semibold text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300">
                {{ t('billingRates.liveRates') }}
              </span>
            </div>

            <div class="mt-4 grid grid-cols-2 gap-3">
              <div class="border-t border-gray-100 pt-3 dark:border-dark-700">
                <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('billingRates.accessibleGroups') }}</div>
                <div class="mt-1 text-2xl font-semibold text-gray-900 dark:text-white">{{ formatCount(groups.length) }}</div>
              </div>
              <div class="border-t border-gray-100 pt-3 dark:border-dark-700">
                <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('billingRates.visibleModels') }}</div>
                <div class="mt-1 text-2xl font-semibold text-gray-900 dark:text-white">{{ formatCount(models.length) }}</div>
              </div>
              <div class="border-t border-gray-100 pt-3 dark:border-dark-700">
                <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('billingRates.customRates') }}</div>
                <div class="mt-1 text-2xl font-semibold text-gray-900 dark:text-white">{{ formatCount(customRateCount) }}</div>
              </div>
              <div class="border-t border-gray-100 pt-3 dark:border-dark-700">
                <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('billingRates.platforms') }}</div>
                <div class="mt-1 text-2xl font-semibold text-gray-900 dark:text-white">{{ formatCount(platformCount) }}</div>
              </div>
            </div>

            <div v-if="lowestStandardGroup" class="mt-4 border-t border-gray-100 pt-4 dark:border-dark-700">
              <div class="flex items-start justify-between gap-3">
                <div>
                  <p class="text-xs font-medium text-gray-500 dark:text-gray-400">
                    {{ t('billingRates.lowestStandardRate') }}
                  </p>
                  <p class="mt-1 text-2xl font-bold text-emerald-700 dark:text-emerald-300">
                    {{ formatRate(lowestStandardGroup.effective_multiplier) }}
                  </p>
                </div>
                <GroupBadge
                  :name="lowestStandardGroup.name"
                  :platform="lowestStandardGroup.platform as GroupPlatform"
                  :subscription-type="(lowestStandardGroup.subscription_type || 'standard') as SubscriptionType"
                  :rate-multiplier="lowestStandardGroup.default_rate_multiplier"
                  :user-rate-multiplier="lowestStandardGroup.custom_rate_multiplier ?? null"
                  always-show-rate
                />
              </div>
              <p class="mt-3 text-xs leading-5 text-gray-500 dark:text-gray-400">
                {{ lowestImageGroup && lowestImageGroup.id !== lowestStandardGroup.id
                  ? t('billingRates.lowestImageRateHint', { rate: formatRate(lowestImageGroup.effective_image_multiplier), group: lowestImageGroup.name })
                  : t('billingRates.lowestStandardRateHint') }}
              </p>
            </div>
            <div v-else class="mt-4 border-t border-gray-100 pt-4 text-sm text-gray-500 dark:border-dark-700 dark:text-gray-400">
              {{ t('billingRates.emptyGroups') }}
            </div>
          </aside>
        </div>
      </section>

      <section class="space-y-4">
        <div class="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
          <div>
            <h2 class="text-base font-semibold text-gray-900 dark:text-white">
              {{ t('billingRates.modelCompareTitle') }}
            </h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
              {{ t('billingRates.modelCompareDescription') }}
            </p>
          </div>
          <div class="flex flex-col gap-3 sm:flex-row sm:items-center">
            <div class="relative w-full sm:w-80">
              <Icon
                name="search"
                size="md"
                class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400 dark:text-gray-500"
              />
              <input
                v-model="searchQuery"
                type="text"
                :placeholder="t('billingRates.searchPlaceholder')"
                class="input pl-10"
              />
            </div>
            <RouterLink to="/usage" class="btn btn-secondary justify-center">
              <Icon name="chart" size="md" />
              <span class="ml-2">{{ t('billingRates.viewUsage') }}</span>
            </RouterLink>
          </div>
        </div>

        <div class="rounded-lg border border-amber-100 bg-amber-50/70 px-5 py-3 text-sm text-amber-800 dark:border-amber-900/40 dark:bg-amber-900/20 dark:text-amber-200">
          <div class="flex items-start gap-2">
            <Icon name="infoCircle" size="sm" class="mt-0.5 flex-shrink-0" />
            <span>{{ t('billingRates.formulaHint') }}</span>
          </div>
        </div>

        <div v-if="loading" class="card px-5 py-10 text-center">
          <Icon name="refresh" size="lg" class="inline-block animate-spin text-gray-400" />
        </div>
        <div v-else-if="filteredModelGroups.length === 0" class="card px-5 py-12 text-center">
          <Icon name="inbox" size="xl" class="mx-auto mb-3 text-gray-400" />
          <p class="text-sm text-gray-500 dark:text-gray-400">
            {{ modelEmptyTitle }}
          </p>
          <p v-if="!searchQuery.trim()" class="mt-1 text-xs text-gray-400 dark:text-gray-500">
            {{ t('billingRates.emptyModelsHint') }}
          </p>
        </div>
        <div v-else class="space-y-4">
          <article
            v-for="modelGroup in filteredModelGroups"
            :key="modelGroup.model"
            class="card overflow-hidden"
          >
            <div class="flex flex-col gap-4 border-b border-gray-100 px-5 py-4 dark:border-dark-700 xl:flex-row xl:items-start xl:justify-between">
              <div class="min-w-0">
                <div class="flex flex-wrap items-center gap-2">
                  <h3 class="text-base font-semibold text-gray-900 dark:text-white">
                    {{ modelGroup.model }}
                  </h3>
                  <span class="rounded bg-emerald-50 px-2 py-1 text-xs font-semibold text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300">
                    {{ t('billingRates.modelLowestMultiplier', { rate: formatRate(modelGroup.minMultiplier) }) }}
                  </span>
                </div>
                <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                  {{ t('billingRates.modelVariantSummary', { count: modelGroup.rows.length, groups: modelGroup.groupCount }) }}
                </p>
              </div>
              <div class="flex flex-wrap gap-1.5">
                <span
                  v-for="platform in modelGroup.platforms"
                  :key="platform"
                  :class="[
                    'inline-flex items-center gap-1.5 rounded-md border px-2 py-0.5 text-xs font-medium uppercase',
                    platformClass(platform)
                  ]"
                >
                  <PlatformIcon
                    :platform="platform as GroupPlatform"
                    size="xs"
                  />
                  {{ platform }}
                </span>
              </div>
            </div>

            <div class="divide-y divide-gray-100 dark:divide-dark-800">
              <div
                v-for="(row, index) in modelGroup.rows"
                :key="`${row.channel_name}-${row.platform}-${row.group.id}-${row.model}-${index}`"
                class="grid gap-4 px-5 py-4 transition-colors hover:bg-gray-50/50 dark:hover:bg-dark-800/40 lg:grid-cols-[minmax(210px,1.05fr)_minmax(190px,1fr)_minmax(170px,0.85fr)_minmax(180px,0.9fr)]"
              >
                <div class="min-w-0">
                  <div class="mb-2 text-xs font-medium text-gray-500 dark:text-gray-400">
                    {{ t('billingRates.columns.group') }}
                  </div>
                  <div class="flex flex-wrap items-center gap-2">
                    <GroupBadge
                      :name="row.group.name"
                      :platform="row.group.platform as GroupPlatform"
                      :subscription-type="(row.group.subscription_type || 'standard') as SubscriptionType"
                      :rate-multiplier="row.group.default_rate_multiplier"
                      :user-rate-multiplier="row.group.custom_rate_multiplier ?? null"
                      always-show-rate
                    />
                  </div>
                  <div class="mt-2 flex flex-wrap gap-1.5">
                    <span class="rounded bg-emerald-50 px-2 py-1 text-xs font-semibold text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300">
                      {{ formatRate(row.applied_multiplier) }}
                    </span>
                    <span
                      v-if="row.applied_multiplier === modelGroup.minMultiplier"
                      class="rounded bg-amber-50 px-2 py-1 text-xs font-semibold text-amber-700 dark:bg-amber-900/30 dark:text-amber-200"
                    >
                      {{ t('billingRates.modelBestRate') }}
                    </span>
                    <span
                      class="rounded px-2 py-1 text-xs font-semibold"
                      :class="row.multiplier_type === 'image'
                        ? 'bg-pink-50 text-pink-700 dark:bg-pink-900/30 dark:text-pink-300'
                        : 'bg-gray-100 text-gray-700 dark:bg-dark-700 dark:text-gray-200'"
                    >
                      {{ multiplierTypeLabel(row.multiplier_type) }}
                    </span>
                  </div>
                </div>

                <div>
                  <div class="mb-2 text-xs font-medium text-gray-500 dark:text-gray-400">
                    {{ t('billingRates.columns.billedPrice') }}
                  </div>
                  <PricingLines :lines="pricingLines(row.effective_pricing)" :empty="t('billingRates.noPricing')" strong />
                </div>

                <div>
                  <div class="mb-2 text-xs font-medium text-gray-500 dark:text-gray-400">
                    {{ t('billingRates.columns.basePrice') }}
                  </div>
                  <PricingLines :lines="pricingLines(row.base_pricing)" :empty="t('billingRates.noPricing')" />
                  <div class="mt-2 text-xs text-gray-400 dark:text-gray-500">
                    {{ pricingSourceLabel(row.pricing_source) }}
                  </div>
                </div>

                <div class="min-w-0">
                  <div class="mb-2 text-xs font-medium text-gray-500 dark:text-gray-400">
                    {{ t('billingRates.columns.channel') }}
                  </div>
                  <div class="flex flex-wrap items-center gap-2">
                    <span
                      :class="[
                        'inline-flex items-center gap-1.5 rounded-md border px-2 py-0.5 text-xs font-medium uppercase',
                        platformClass(row.platform)
                      ]"
                    >
                      <PlatformIcon
                        v-if="row.platform"
                        :platform="row.platform as GroupPlatform"
                        size="xs"
                      />
                      {{ row.platform || '-' }}
                    </span>
                    <span
                      class="inline-flex rounded px-2 py-0.5 text-xs font-medium"
                      :class="getBillingModeBadgeClass(row.base_pricing?.billing_mode)"
                    >
                      {{ getBillingModeLabel(row.base_pricing?.billing_mode, t) }}
                    </span>
                  </div>
                  <div class="mt-2 font-medium text-gray-900 dark:text-white">{{ row.channel_name }}</div>
                  <div v-if="row.channel_description" class="mt-1 text-xs leading-5 text-gray-500 dark:text-gray-400">
                    {{ row.channel_description }}
                  </div>
                </div>
              </div>
            </div>
          </article>
        </div>
      </section>
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
import { getBillingModeBadgeClass, getBillingModeLabel } from '@/utils/billingMode'
import { platformBadgeClass } from '@/utils/platformColors'
import {
  BILLING_MODE_IMAGE,
  BILLING_MODE_PER_REQUEST,
  BILLING_MODE_TOKEN
} from '@/constants/channel'

const PricingLines = defineComponent({
  name: 'PricingLines',
  props: {
    lines: {
      type: Array as () => string[],
      required: true
    },
    empty: {
      type: String,
      required: true
    },
    strong: {
      type: Boolean,
      default: false
    }
  },
  setup(props) {
    return () => {
      if (props.lines.length === 0) {
        return h('span', { class: 'text-gray-400 dark:text-gray-500' }, props.empty)
      }
      return h(
        'div',
        { class: 'space-y-1' },
        props.lines.map((line) =>
          h(
            'div',
            {
              class: [
                'whitespace-nowrap text-xs',
                props.strong
                  ? 'font-semibold text-emerald-700 dark:text-emerald-300'
                  : 'text-gray-700 dark:text-gray-300'
              ]
            },
            line
          )
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
let abortController: AbortController | null = null

interface ModelRateGroup {
  model: string
  rows: UserBillingRateModel[]
  minMultiplier: number
  groupCount: number
  platforms: string[]
}

const customRateCount = computed(() =>
  groups.value.filter((group) => group.custom_rate_multiplier != null).length
)

const platformCount = computed(() => {
  const platforms = new Set(groups.value.map((group) => group.platform).filter(Boolean))
  return platforms.size
})

const lowestStandardGroup = computed(() => lowestBy(groups.value, 'effective_multiplier'))
const lowestImageGroup = computed(() => lowestBy(groups.value, 'effective_image_multiplier'))

const filteredModelGroups = computed<ModelRateGroup[]>(() => {
  const q = searchQuery.value.trim().toLowerCase()
  const rows = q ? models.value.filter((row) => modelRateMatchesSearch(row, q)) : models.value
  const byModel = new Map<string, UserBillingRateModel[]>()

  for (const row of rows) {
    const modelName = row.model || '-'
    const bucket = byModel.get(modelName)
    if (bucket) {
      bucket.push(row)
    } else {
      byModel.set(modelName, [row])
    }
  }

  return Array.from(byModel.entries())
    .map(([model, modelRows]) => {
      const sortedRows = [...modelRows].sort(compareModelRateRows)
      return {
        model,
        rows: sortedRows,
        minMultiplier: sortedRows[0]?.applied_multiplier ?? 0,
        groupCount: new Set(sortedRows.map((row) => row.group.id)).size,
        platforms: Array.from(new Set(sortedRows.map((row) => row.platform).filter(Boolean))).sort((a, b) =>
          a.localeCompare(b, undefined, { sensitivity: 'base' })
        )
      }
    })
    .sort((a, b) => a.model.localeCompare(b.model, undefined, { sensitivity: 'base' }))
})

const modelEmptyTitle = computed(() =>
  searchQuery.value.trim() ? t('billingRates.noMatches') : t('billingRates.emptyModels')
)

async function loadRates() {
  if (abortController) {
    abortController.abort()
  }
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
    if (abortController === current) {
      loading.value = false
    }
  }
}

function formatRate(value: number): string {
  return `${formatMultiplier(value)}x`
}

function formatCount(value: number): string {
  return value.toLocaleString()
}

function lowestBy(
  source: UserBillingRateGroup[],
  key: 'effective_multiplier' | 'effective_image_multiplier'
): UserBillingRateGroup | null {
  return source.reduce<UserBillingRateGroup | null>((lowest, group) => {
    if (!lowest || group[key] < lowest[key]) {
      return group
    }
    return lowest
  }, null)
}

function platformClass(platform: string): string {
  if (!platform) {
    return 'border-gray-200 bg-gray-50 text-gray-700 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-300'
  }
  return platformBadgeClass(platform)
}

function modelRateMatchesSearch(row: UserBillingRateModel, query: string): boolean {
  return [
    row.channel_name,
    row.channel_description,
    row.platform,
    row.group.name,
    row.group.platform,
    row.model,
    getBillingModeLabel(row.base_pricing?.billing_mode, t),
    pricingSourceLabel(row.pricing_source)
  ]
    .filter(Boolean)
    .some((value) => String(value).toLowerCase().includes(query))
}

function compareModelRateRows(a: UserBillingRateModel, b: UserBillingRateModel): number {
  const multiplierDiff = a.applied_multiplier - b.applied_multiplier
  if (multiplierDiff !== 0) return multiplierDiff

  const left = `${a.group.name} ${a.channel_name} ${a.platform}`
  const right = `${b.group.name} ${b.channel_name} ${b.platform}`
  return left.localeCompare(right, undefined, { sensitivity: 'base' })
}

function pricingSourceLabel(source: string): string {
  switch (source) {
    case 'channel':
      return t('billingRates.pricingSourceChannel')
    case 'litellm':
      return t('billingRates.pricingSourceLiteLLM')
    case 'fallback':
      return t('billingRates.pricingSourceFallback')
    case 'display':
      return t('billingRates.pricingSourceDisplay')
    default:
      return source || t('billingRates.pricingSourceUnknown')
  }
}

function multiplierTypeLabel(type: string): string {
  if (type === 'image') return t('billingRates.multiplierImage')
  return t('billingRates.multiplierStandard')
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

onBeforeUnmount(() => {
  abortController?.abort()
})
</script>
