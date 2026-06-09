<template>
  <div class="conversation-panel">
    <div class="conversation-header">
      <div>
        <p class="conversation-kicker">{{ t('imageStudio.conversations') }}</p>
        <h2>{{ activeTitle }}</h2>
      </div>
    </div>

    <button
      type="button"
      class="new-conversation-button"
      :disabled="creating"
      @click="$emit('create')"
    >
      <Icon name="plus" size="sm" class="mr-1.5" />
      {{ t('imageStudio.newConversation') }}
    </button>

    <div class="quick-actions">
      <button
        type="button"
        class="quick-action"
        :class="{ 'quick-action-active': activeConversationId === null }"
        @click="$emit('select', null)"
      >
        <span class="quick-action-icon">
          <Icon name="grid" size="sm" />
        </span>
        <span class="min-w-0 flex-1 truncate">{{ t('imageStudio.allGenerations') }}</span>
      </button>

      <button
        type="button"
        class="quick-action quick-action-danger"
        :disabled="loading || conversations.length === 0"
        @click="$emit('clear')"
      >
        <span class="quick-action-icon">
          <Icon name="trash" size="sm" />
        </span>
        <span class="min-w-0 flex-1 truncate">{{ t('imageStudio.clearHistory') }}</span>
      </button>
    </div>

    <div class="history-section">
      <div class="history-heading">
        <span>{{ t('imageStudio.conversationHistory') }}</span>
        <span>{{ conversations.length }}</span>
      </div>

      <div class="conversation-list">
        <div v-if="loading && conversations.length === 0" class="space-y-2 pt-1">
          <div
            v-for="i in 4"
            :key="i"
            class="h-11 animate-pulse rounded-xl bg-gray-100 dark:bg-dark-700"
          ></div>
        </div>

        <p
          v-else-if="conversations.length === 0"
          class="px-3 py-8 text-center text-xs text-gray-400 dark:text-dark-500"
        >
          {{ t('imageStudio.noConversations') }}
        </p>

        <div
          v-for="conv in conversations"
          :key="conv.id"
          class="conversation-row group"
          :class="{ 'conversation-row-active': conv.id === activeConversationId }"
        >
          <input
            v-if="editingId === conv.id"
            ref="renameInput"
            v-model="editingTitle"
            type="text"
            class="rename-input"
            @keydown.enter.prevent="commitRename(conv)"
            @keydown.esc.prevent="cancelRename"
            @blur="commitRename(conv)"
          />

          <template v-else>
            <button
              type="button"
              class="conversation-main"
              @click="$emit('select', conv.id)"
            >
              <span class="conversation-icon">
                <Icon name="chat" size="sm" />
              </span>
              <span class="conversation-title">{{ conv.title || t('imageStudio.untitled') }}</span>
            </button>

            <div class="conversation-actions">
              <button
                type="button"
                class="conversation-action"
                :title="t('common.rename')"
                @click.stop="startRename(conv)"
              >
                <Icon name="edit" size="xs" />
              </button>
              <button
                type="button"
                class="conversation-action conversation-action-danger"
                :title="t('common.delete')"
                @click.stop="$emit('delete', conv)"
              >
                <Icon name="trash" size="xs" />
              </button>
            </div>
          </template>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, nextTick, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ImageStudioConversation } from '@/types'
import Icon from '@/components/icons/Icon.vue'

const props = defineProps<{
  conversations: ImageStudioConversation[]
  activeConversationId: number | null
  loading?: boolean
  creating?: boolean
}>()

const emit = defineEmits<{
  (e: 'create'): void
  (e: 'select', id: number | null): void
  (e: 'rename', payload: { id: number; title: string }): void
  (e: 'delete', conversation: ImageStudioConversation): void
  (e: 'clear'): void
}>()

const { t } = useI18n()

const editingId = ref<number | null>(null)
const editingTitle = ref('')
const renameInput = ref<HTMLInputElement | HTMLInputElement[] | null>(null)

const activeTitle = computed(() => {
  if (props.activeConversationId === null) return t('imageStudio.allGenerations')
  const conv = props.conversations.find((item) => item.id === props.activeConversationId)
  return conv?.title || t('imageStudio.untitled')
})

async function startRename(conv: ImageStudioConversation) {
  editingId.value = conv.id
  editingTitle.value = conv.title || ''
  await nextTick()
  const el = Array.isArray(renameInput.value) ? renameInput.value[0] : renameInput.value
  el?.focus()
  el?.select()
}

function cancelRename() {
  editingId.value = null
  editingTitle.value = ''
}

function commitRename(conv: ImageStudioConversation) {
  if (editingId.value === null) return
  const title = editingTitle.value.trim()
  const id = conv.id
  editingId.value = null
  if (title && title !== conv.title) {
    emit('rename', { id, title })
  }
}
</script>

<style scoped>
.conversation-panel {
  @apply flex h-full min-h-0 flex-col rounded-2xl border border-gray-100 bg-white/85 p-3 shadow-sm;
  @apply dark:border-dark-700/50 dark:bg-dark-800/75;
}

.conversation-header {
  @apply flex-shrink-0 px-1 pb-3;
}

.conversation-kicker {
  @apply text-[11px] font-semibold uppercase tracking-[0.14em] text-gray-500 dark:text-gray-400;
}

.conversation-header h2 {
  @apply mt-1 truncate text-sm font-semibold text-gray-900 dark:text-white;
}

.new-conversation-button {
  @apply mb-3 inline-flex h-10 w-full flex-shrink-0 items-center justify-center rounded-xl bg-primary-600 px-4 text-sm font-semibold text-white shadow-sm transition-colors;
  @apply hover:bg-primary-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/40;
  @apply disabled:cursor-not-allowed disabled:opacity-60;
}

.quick-actions {
  @apply flex-shrink-0 space-y-1.5 rounded-xl border border-gray-100 bg-gray-50/80 p-1.5 dark:border-dark-700/70 dark:bg-dark-900/40;
}

.quick-action {
  @apply flex h-9 w-full items-center gap-2 rounded-lg px-2.5 text-left text-sm font-medium text-gray-600 transition-colors;
  @apply hover:bg-white hover:text-gray-900 dark:text-gray-300 dark:hover:bg-dark-700 dark:hover:text-white;
  @apply disabled:cursor-not-allowed disabled:opacity-45;
}

.quick-action-active {
  @apply bg-white text-primary-700 shadow-sm dark:bg-primary-900/25 dark:text-primary-300;
}

.quick-action-danger {
  @apply hover:text-red-600 dark:hover:text-red-300;
}

.quick-action-icon {
  @apply inline-flex h-7 w-7 flex-shrink-0 items-center justify-center rounded-lg bg-white text-gray-400 dark:bg-dark-800 dark:text-gray-400;
}

.history-section {
  @apply mt-3 flex min-h-0 flex-1 flex-col rounded-xl border border-gray-100 bg-white/70 p-2 dark:border-dark-700/60 dark:bg-dark-900/35;
}

.history-heading {
  @apply mb-2 flex flex-shrink-0 items-center justify-between px-1 text-[11px] font-semibold uppercase tracking-normal text-gray-400 dark:text-dark-400;
}

.conversation-list {
  @apply min-h-0 flex-1 space-y-1 overflow-y-auto pr-1;
}

.conversation-row {
  @apply relative flex min-h-10 items-center rounded-xl transition-colors;
  @apply hover:bg-gray-50 dark:hover:bg-dark-700/70;
}

.conversation-row-active {
  @apply bg-primary-50 dark:bg-primary-900/25;
}

.conversation-main {
  @apply flex min-w-0 flex-1 items-center gap-2 px-2.5 py-2 text-left text-sm text-gray-700 dark:text-gray-300;
}

.conversation-row-active .conversation-main {
  @apply font-semibold text-primary-700 dark:text-primary-300;
}

.conversation-icon {
  @apply inline-flex h-7 w-7 flex-shrink-0 items-center justify-center rounded-lg bg-gray-100 text-gray-400 dark:bg-dark-800 dark:text-gray-400;
}

.conversation-title {
  @apply min-w-0 flex-1 truncate;
}

.conversation-actions {
  @apply absolute right-1.5 flex items-center gap-0.5 opacity-0 transition-opacity group-hover:opacity-100;
}

.conversation-action {
  @apply flex h-6 w-6 items-center justify-center rounded-md text-gray-400 transition-colors hover:bg-white hover:text-primary-600;
  @apply dark:hover:bg-dark-600 dark:hover:text-primary-400;
}

.conversation-action-danger {
  @apply hover:text-red-600 dark:hover:text-red-400;
}

.rename-input {
  @apply w-full rounded-xl border border-primary-300 bg-white px-3 py-2 text-sm text-gray-900 outline-none focus:ring-1 focus:ring-primary-400;
  @apply dark:border-primary-600 dark:bg-dark-800 dark:text-white;
}
</style>
