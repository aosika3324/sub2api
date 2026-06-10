<template>
  <AppLayout>
    <div class="space-y-6">
      <div class="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-4">
        <div class="card p-4">
          <div class="flex items-center gap-3">
            <div class="rounded-lg bg-sky-100 p-2 dark:bg-sky-900/30">
              <Icon name="key" size="md" class="text-sky-600 dark:text-sky-400" />
            </div>
            <div class="min-w-0">
              <p class="text-xs font-medium text-gray-500 dark:text-gray-400">
                {{ t('billingRates.accessibleGroups') }}
              </p>
              <p class="text-xl font-bold text-gray-900 dark:text-white">
                {{ groups.length.toLocaleString() }}
              </p>
            </div>
          </div>
        </div>

        <div class="card p-4">
          <div class="flex items-center gap-3">
            <div class="rounded-lg bg-emerald-100 p-2 dark:bg-emerald-900/30">
              <Icon name="database" size="md" class="text-emerald-600 dark:text-emerald-400" />
            </div>
            <div class="min-w-0">
              <p class="text-xs font-medium text-gray-500 dark:text-gray-400">
                {{ t('billingRates.visibleModels') }}
              </p>
              <p class="text-xl font-bold text-gray-900 dark:text-white">
                {{ models.length.toLocaleString() }}
              </p>
            </div>
          </div>
        </div>

        <div class="card p-4">
          <div class="flex items-center gap-3">
            <div class="rounded-lg bg-violet-100 p-2 dark:bg-violet-900/30">
              <Icon name="badge" size="md" class="text-violet-600 dark:text-violet-400" />
            </div>
            <div class="min-w-0">
              <p class="text-xs font-medium text-gray-500 dark:text-gray-400">
                {{ t('billingRates.customRates') }}
              </p>
              <p class="text-xl font-bold text-gray-900 dark:text-white">
                {{ customRateCount.toLocaleString() }}
              </p>
            </div>
          </div>
        </div>

        <div class="card p-4">
          <div class="flex items-center gap-3">
            <div class="rounded-lg bg-amber-100 p-2 dark:bg-amber-900/30">
              <Icon name="calculator" size="md" class="text-amber-600 dark:text-amber-400" />
            </div>
            <div class="min-w-0">
              <p class="text-xs font-medium text-gray-500 dark:text-gray-400">
                {{ t('billingRates.formulaTitle') }}
              </p>
              <p class="truncate text-sm font-semibold text-gray-900 dark:text-white">
                {{ t('billingRates.formula') }}
              </p>
            </div>
          </div>
        </div>
      </div>

      <section class="card overflow-hidden">
        <div class="flex flex-col gap-3 border-b border-gray-100 px-5 py-4 dark:border-dark-700 md:flex-row md:items-center md:justify-between">
          <div>
            <h2 class="text-base font-semibold text-gray-900 dark:text-white">
              {{ t('billingRates.groupsTitle') }}
            </h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
              {{ t('billingRates.groupsDescription') }}
            </p>
          </div>
          <button
            @click="loadRates"
            :disabled="loading"
            class="btn btn-secondary w-full md:w-auto"
            :title="t('common.refresh')"
          >
            <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
            <span class="ml-2">{{ t('billingRates.refresh') }}</span>
          </button>
        </div>

        <div class="overflow-x-auto">
          <table class="w-full min-w-[900px] border-collapse text-sm">
            <thead>
              <tr class="border-b border-gray-100 bg-gray-50/70 text-xs font-medium uppercase text-gray-500 dark:border-dark-700 dark:bg-dark-800/60 dark:text-gray-400">
                <th class="px-5 py-3 text-left">{{ t('billingRates.columns.platform') }}</th>
                <th class="px-5 py-3 text-left">{{ t('billingRates.columns.group') }}</th>
                <th class="px-5 py-3 text-left">{{ t('billingRates.columns.defaultRate') }}</th>
                <th class="px-5 py-3 text-left">{{ t('billingRates.columns.customRate') }}</th>
                <th class="px-5 py-3 text-left">{{ t('billingRates.columns.effectiveRate') }}</th>
                <th class="px-5 py-3 text-left">{{ t('billingRates.columns.imageRate') }}</th>
                <th class="px-5 py-3 text-left">{{ t('billingRates.columns.scope') }}</th>
              </tr>
            </thead>
            <tbody v-if="loading">
              <tr>
                <td colspan="7" class="px-5 py-10 text-center">
                  <Icon name="refresh" size="lg" class="inline-block animate-spin text-gray-400" />
                </td>
              </tr>
            </tbody>
            <tbody v-else-if="groups.length === 0">
              <tr>
                <td colspan="7" class="px-5 py-12 text-center">
                  <Icon name="inbox" size="xl" class="mx-auto mb-3 text-gray-400" />
                  <p class="text-sm text-gray-500 dark:text-gray-400">
                    {{ t('billingRates.emptyGroups') }}
                  </p>
                </td>
              </tr>
            </tbody>
            <tbody v-else>
              <tr
                v-for="group in groups"
                :key="group.id"
                class="border-b border-gray-100 transition-colors last:border-b-0 hover:bg-gray-50/50 dark:border-dark-800 dark:hover:bg-dark-800/40"
              >
                <td class="px-5 py-4">
                  <span
                    :class="[
                      'inline-flex items-center gap-1.5 rounded-md border px-2 py-0.5 text-xs font-medium uppercase',
                      platformClass(group.platform)
                    ]"
                  >
                    <PlatformIcon
                      v-if="group.platform"
                      :platform="group.platform as GroupPlatform"
                      size="xs"
                    />
                    {{ group.platform || '-' }}
                  </span>
                </td>
                <td class="px-5 py-4">
                  <GroupBadge
                    :name="group.name"
                    :platform="group.platform as GroupPlatform"
                    :subscription-type="(group.subscription_type || 'standard') as SubscriptionType"
                    :rate-multiplier="group.default_rate_multiplier"
                    :user-rate-multiplier="group.custom_rate_multiplier ?? null"
                    always-show-rate
                  />
                </td>
                <td class="px-5 py-4 font-medium text-gray-900 dark:text-white">
                  {{ formatRate(group.default_rate_multiplier) }}
                </td>
                <td class="px-5 py-4">
                  <span v-if="group.custom_rate_multiplier != null" class="font-medium text-violet-600 dark:text-violet-400">
                    {{ formatRate(group.custom_rate_multiplier) }}
                  </span>
                  <span v-else class="text-gray-400 dark:text-gray-500">
                    {{ t('billingRates.noCustomRate') }}
                  </span>
                </td>
                <td class="px-5 py-4">
                  <span class="rounded bg-emerald-50 px-2 py-1 text-xs font-semibold text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300">
                    {{ formatRate(group.effective_multiplier) }}
                  </span>
                </td>
                <td class="px-5 py-4">
                  <div class="flex flex-col gap-1">
                    <span
                      class="w-fit rounded px-2 py-1 text-xs font-semibold"
                      :class="group.image_rate_independent
                        ? 'bg-pink-50 text-pink-700 dark:bg-pink-900/30 dark:text-pink-300'
                        : 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300'"
                    >
                      {{ formatRate(group.effective_image_multiplier) }}
                    </span>
                    <span class="text-xs text-gray-400 dark:text-gray-500">
                      {{ group.image_rate_independent ? t('billingRates.imageRateIndependent') : t('billingRates.imageRateSame') }}
                    </span>
                  </div>
                </td>
                <td class="px-5 py-4">
                  <div class="flex flex-wrap gap-1.5">
                    <span
                      v-for="badge in scopeLabels(group)"
                      :key="badge"
                      class="rounded bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-600 dark:bg-dark-700 dark:text-gray-300"
                    >
                      {{ badge }}
                    </span>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>

      <section class="card overflow-hidden">
        <div class="flex flex-col gap-4 border-b border-gray-100 px-5 py-4 dark:border-dark-700 lg:flex-row lg:items-center lg:justify-between">
          <div>
            <h2 class="text-base font-semibold text-gray-900 dark:text-white">
              {{ t('billingRates.modelsTitle') }}
            </h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
              {{ t('billingRates.modelsDescription') }}
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

        <div class="border-b border-amber-100 bg-amber-50/70 px-5 py-3 text-sm text-amber-800 dark:border-amber-900/40 dark:bg-amber-900/20 dark:text-amber-200">
          <div class="flex items-start gap-2">
            <Icon name="infoCircle" size="sm" class="mt-0.5 flex-shrink-0" />
            <span>{{ t('billingRates.formulaHint') }}</span>
          </div>
        </div>

        <div class="overflow-x-auto">
          <table class="w-full min-w-[1160px] border-collapse text-sm">
            <thead>
              <tr class="border-b border-gray-100 bg-gray-50/70 text-xs font-medium uppercase text-gray-500 dark:border-dark-700 dark:bg-dark-800/60 dark:text-gray-400">
                <th class="px-5 py-3 text-left">{{ t('billingRates.columns.channel') }}</th>
                <th class="px-5 py-3 text-left">{{ t('billingRates.columns.platform') }}</th>
                <th class="px-5 py-3 text-left">{{ t('billingRates.columns.group') }}</th>
                <th class="px-5 py-3 text-left">{{ t('billingRates.columns.model') }}</th>
                <th class="px-5 py-3 text-left">{{ t('billingRates.columns.billingMode') }}</th>
                <th class="px-5 py-3 text-left">{{ t('billingRates.columns.basePrice') }}</th>
                <th class="px-5 py-3 text-left">{{ t('billingRates.columns.multiplier') }}</th>
                <th class="px-5 py-3 text-left">{{ t('billingRates.columns.billedPrice') }}</th>
              </tr>
            </thead>
            <tbody v-if="loading">
              <tr>
                <td colspan="8" class="px-5 py-10 text-center">
                  <Icon name="refresh" size="lg" class="inline-block animate-spin text-gray-400" />
                </td>
              </tr>
            </tbody>
            <tbody v-else-if="filteredModels.length === 0">
              <tr>
                <td colspan="8" class="px-5 py-12 text-center">
                  <Icon name="inbox" size="xl" class="mx-auto mb-3 text-gray-400" />
                  <p class="text-sm text-gray-500 dark:text-gray-400">
                    {{ modelEmptyTitle }}
                  </p>
                  <p v-if="!searchQuery.trim()" class="mt-1 text-xs text-gray-400 dark:text-gray-500">
                    {{ t('billingRates.emptyModelsHint') }}
                  </p>
                </td>
              </tr>
            </tbody>
            <tbody v-else>
              <tr
                v-for="(row, index) in filteredModels"
                :key="`${row.channel_name}-${row.platform}-${row.group.id}-${row.model}-${index}`"
                class="border-b border-gray-100 transition-colors last:border-b-0 hover:bg-gray-50/50 dark:border-dark-800 dark:hover:bg-dark-800/40"
              >
                <td class="px-5 py-4 align-top">
                  <div class="font-medium text-gray-900 dark:text-white">{{ row.channel_name }}</div>
                  <div v-if="row.channel_description" class="mt-1 max-w-[220px] text-xs text-gray-500 dark:text-gray-400">
                    {{ row.channel_description }}
                  </div>
                </td>
                <td class="px-5 py-4 align-top">
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
                </td>
                <td class="px-5 py-4 align-top">
                  <GroupBadge
                    :name="row.group.name"
                    :platform="row.group.platform as GroupPlatform"
                    :subscription-type="(row.group.subscription_type || 'standard') as SubscriptionType"
                    :rate-multiplier="row.group.default_rate_multiplier"
                    :user-rate-multiplier="row.group.custom_rate_multiplier ?? null"
                    always-show-rate
                  />
                </td>
                <td class="px-5 py-4 align-top font-medium text-gray-900 dark:text-white">
                  {{ row.model }}
                </td>
                <td class="px-5 py-4 align-top">
                  <span
                    class="inline-flex rounded px-2 py-0.5 text-xs font-medium"
                    :class="getBillingModeBadgeClass(row.base_pricing?.billing_mode)"
                  >
                    {{ getBillingModeLabel(row.base_pricing?.billing_mode, t) }}
                  </span>
                  <div class="mt-1 text-xs text-gray-400 dark:text-gray-500">
                    {{ pricingSourceLabel(row.pricing_source) }}
                  </div>
                </td>
                <td class="px-5 py-4 align-top">
                  <PricingLines :lines="pricingLines(row.base_pricing)" :empty="t('billingRates.noPricing')" />
                </td>
                <td class="px-5 py-4 align-top">
                  <div class="flex flex-col gap-1">
                    <span
                      class="w-fit rounded px-2 py-1 text-xs font-semibold"
                      :class="row.multiplier_type === 'image'
                        ? 'bg-pink-50 text-pink-700 dark:bg-pink-900/30 dark:text-pink-300'
                        : 'bg-gray-100 text-gray-700 dark:bg-dark-700 dark:text-gray-200'"
                    >
                      {{ formatRate(row.applied_multiplier) }}
                    </span>
                    <span class="text-xs text-gray-400 dark:text-gray-500">
                      {{ multiplierTypeLabel(row.multiplier_type) }}
                    </span>
                  </div>
                </td>
                <td class="px-5 py-4 align-top">
                  <PricingLines :lines="pricingLines(row.effective_pricing)" :empty="t('billingRates.noPricing')" strong />
                </td>
              </tr>
            </tbody>
          </table>
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

const customRateCount = computed(() =>
  groups.value.filter((group) => group.custom_rate_multiplier != null).length
)

const filteredModels = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  if (!q) return models.value
  return models.value.filter((row) =>
    [
      row.channel_name,
      row.channel_description,
      row.platform,
      row.group.name,
      row.model
    ]
      .filter(Boolean)
      .some((value) => String(value).toLowerCase().includes(q))
  )
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

function platformClass(platform: string): string {
  if (!platform) {
    return 'border-gray-200 bg-gray-50 text-gray-700 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-300'
  }
  return platformBadgeClass(platform)
}

function scopeLabels(group: UserBillingRateGroup): string[] {
  const labels = [group.is_exclusive ? t('billingRates.exclusive') : t('billingRates.public')]
  if (group.subscription_type === 'subscription') {
    labels.push(t('billingRates.subscription'))
  }
  return labels
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
