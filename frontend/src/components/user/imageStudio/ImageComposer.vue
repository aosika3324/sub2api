<template>
  <div class="card p-4">
    <!-- No usable group hint -->
    <div
      v-if="!loadingGroups && imageGroups.length === 0"
      class="flex items-center gap-2 rounded-lg bg-amber-50 px-3 py-2 text-sm text-amber-700 dark:bg-amber-900/20 dark:text-amber-300"
    >
      <Icon name="exclamationTriangle" size="sm" class="flex-shrink-0" />
      <span>{{ t('imageStudio.noImageGroupHint') }}</span>
    </div>

    <!-- Prompt -->
    <textarea
      v-model="prompt"
      :rows="3"
      :disabled="disabled"
      class="input resize-none"
      :placeholder="t('imageStudio.promptPlaceholder')"
      @keydown.ctrl.enter.prevent="submit"
      @keydown.meta.enter.prevent="submit"
    ></textarea>

    <!-- Controls row -->
    <div class="mt-3 flex flex-wrap items-end gap-3">
      <!-- Group -->
      <div class="min-w-[160px] flex-1">
        <label class="composer-label">{{ t('imageStudio.group') }}</label>
        <Select
          v-model="groupId"
          :options="groupOptions"
          :disabled="disabled || imageGroups.length === 0"
          :placeholder="t('imageStudio.selectGroup')"
        />
      </div>

      <!-- Model -->
      <div class="w-[150px]">
        <label class="composer-label">{{ t('imageStudio.model') }}</label>
        <Select v-model="model" :options="modelOptions" :disabled="disabled" />
      </div>

      <!-- Size -->
      <div class="w-[140px]">
        <label class="composer-label">{{ t('imageStudio.size') }}</label>
        <Select v-model="size" :options="sizeOptions" :disabled="disabled" />
      </div>

      <!-- Quality -->
      <div class="w-[130px]">
        <label class="composer-label">{{ t('imageStudio.quality') }}</label>
        <Select v-model="quality" :options="qualityOptions" :disabled="disabled" />
      </div>

      <!-- Count -->
      <div class="w-[100px]">
        <label class="composer-label">{{ t('imageStudio.count') }}</label>
        <Select v-model="n" :options="countOptions" :disabled="disabled" />
      </div>

      <!-- Generate -->
      <button
        type="button"
        class="btn btn-primary ml-auto h-[42px] min-w-[120px] justify-center"
        :disabled="!canGenerate"
        @click="submit"
      >
        <template v-if="generating">
          <span
            class="mr-2 h-4 w-4 animate-spin rounded-full border-2 border-white border-t-transparent"
          ></span>
          {{ t('imageStudio.generatingShort') }}
        </template>
        <template v-else>
          <Icon name="sparkles" size="sm" class="mr-1.5" />
          {{ t('imageStudio.generate') }}
        </template>
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { Group } from '@/types'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'

export interface ComposerSubmitPayload {
  group_id: number
  prompt: string
  model: string
  size: string
  quality: string
  n: number
}

const props = defineProps<{
  groups: Group[]
  loadingGroups?: boolean
  generating?: boolean
}>()

const emit = defineEmits<{
  (e: 'generate', payload: ComposerSubmitPayload): void
}>()

const { t } = useI18n()

// Only groups that allow image generation are usable.
const imageGroups = computed(() =>
  props.groups.filter((g) => g.allow_image_generation && g.status === 'active')
)

const prompt = ref('')
const groupId = ref<number | null>(null)
const model = ref('gpt-image-1')
const size = ref('1024x1024')
const quality = ref('auto')
const n = ref(1)

const groupOptions = computed(() =>
  imageGroups.value.map((g) => ({ value: g.id, label: g.name }))
)

const modelOptions = [
  { value: 'gpt-image-1', label: 'gpt-image-1' },
  { value: 'dall-e-3', label: 'dall-e-3' },
]

const sizeOptions = [
  { value: '1024x1024', label: '1024 × 1024' },
  { value: '1024x1536', label: '1024 × 1536' },
  { value: '1536x1024', label: '1536 × 1024' },
]

const qualityOptions = computed(() => [
  { value: 'auto', label: t('imageStudio.qualityAuto') },
  { value: 'low', label: t('imageStudio.qualityLow') },
  { value: 'medium', label: t('imageStudio.qualityMedium') },
  { value: 'high', label: t('imageStudio.qualityHigh') },
])

const countOptions = [1, 2, 3, 4].map((v) => ({ value: v, label: String(v) }))

// Default the group selection to the first usable group.
watch(
  imageGroups,
  (list) => {
    if (groupId.value === null && list.length > 0) {
      groupId.value = list[0].id
    } else if (groupId.value !== null && !list.some((g) => g.id === groupId.value)) {
      // Previously selected group is no longer usable — reset.
      groupId.value = list.length > 0 ? list[0].id : null
    }
  },
  { immediate: true }
)

const disabled = computed(() => props.generating === true)

const canGenerate = computed(
  () =>
    !disabled.value &&
    prompt.value.trim().length > 0 &&
    groupId.value !== null &&
    imageGroups.value.length > 0
)

function submit() {
  if (!canGenerate.value || groupId.value === null) return
  emit('generate', {
    group_id: groupId.value,
    prompt: prompt.value.trim(),
    model: model.value,
    size: size.value,
    quality: quality.value,
    n: n.value,
  })
}

// Allow the parent to clear the prompt after a successful generate.
function resetPrompt() {
  prompt.value = ''
}

defineExpose({ resetPrompt })
</script>

<style scoped>
.composer-label {
  @apply mb-1 block text-xs font-medium text-gray-500 dark:text-gray-400;
}
</style>
