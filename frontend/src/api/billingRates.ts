import { apiClient } from './client'
import type { UserSupportedModelPricing } from './channels'
import type { GroupPlatform, SubscriptionType } from '@/types'

export interface UserBillingRateGroup {
  id: number
  name: string
  platform: GroupPlatform | string
  subscription_type: SubscriptionType | string
  default_rate_multiplier: number
  custom_rate_multiplier?: number | null
  effective_multiplier: number
  image_rate_independent: boolean
  image_rate_multiplier: number
  effective_image_multiplier: number
  is_exclusive: boolean
}

export interface UserBillingRateModel {
  channel_name: string
  channel_description: string
  platform: string
  group: UserBillingRateGroup
  model: string
  base_pricing: UserSupportedModelPricing | null
  effective_pricing: UserSupportedModelPricing | null
  pricing_source: 'channel' | 'litellm' | 'fallback' | 'display' | string
  pricing_kind: 'unit_price_table' | string
  applied_multiplier: number
  multiplier_type: 'standard' | 'image' | string
}

export interface UserBillingRatesResponse {
  groups: UserBillingRateGroup[]
  models: UserBillingRateModel[]
}

export async function getBillingRates(options?: { signal?: AbortSignal }): Promise<UserBillingRatesResponse> {
  const { data } = await apiClient.get<UserBillingRatesResponse>('/billing/rates', {
    signal: options?.signal
  })
  return {
    groups: data?.groups ?? [],
    models: data?.models ?? []
  }
}

export const billingRatesAPI = { getBillingRates }

export default billingRatesAPI
