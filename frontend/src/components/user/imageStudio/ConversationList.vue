<template>
  <div class="flex h-full flex-col">
    <!-- Header -->
    <div class="flex items-center justify-between px-1 pb-3">
      <h2
        class="text-[11px] font-semibold uppercase tracking-[0.14em] text-gray-500 dark:text-gray-400"
      >
        {{ t('imageStudio.conversations') }}
      </h2>
    </div>

    <!-- New conversation -->
    <button
      type="button"
      class="btn btn-primary mb-3 w-full justify-center"
      :disabled="creating"
      @click="$emit('create')"
    >
      <Icon name="plus" size="sm" class="mr-1.5" />
      {{ t('imageStudio.newConversation') }}
    </button>

    <!-- Global gallery shortcut -->
    <button
      type="button"
      class="mb-2 flex w-full items-center gap-2 rounded-lg px-3 py-2 text-sm transition-colors"
      :class="
        activeConversationId === null
          ? 'bg-primary-50 font-medium text-primary-700 dark:bg-primary-900/20 dark:text-primary-300'
          : 'text-gray-600 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-dark-700'
      "
      @click="$emit('select', null)"
    >
      <Icon name="grid" size="sm" class="flex-shrink-0 opacity-70" />
      <span class="truncate">{{ t('imageStudio.allGenerations') }}</span>
    </button>

    <!-- List -->
    <div class="-mx-1 min-h-0 flex-1 space-y-1 overflow-y-auto px-1">
      <!-- Loading -->
      <div v-if="loading && conversations.length === 0" class="space-y-2 pt-1">
        <div
          v-for="i in 4"
          :key="i"
          class="h-10 animate-pulse rounded-lg bg-gray-100 dark:bg-dark-700"
        ></div>
      </div>

      <!-- Empty -->
      <p
        v-else-if="conversations.length === 0"
        class="px-3 py-6 text-center text-xs text-gray-400 dark:text-dark-500"
      >
        {{ t('imageStudio.noConversations') }}
      </p>

      <!-- Items -->
      <div
        v-for="conv in conversations"
        :key="conv.id"
        class="group relative flex items-center rounded-lg transition-colors"
        :class="
          conv.id === activeConversationId
            ? 'bg-primary-50 dark:bg-primary-900/20'
            : 'hover:bg-gray-100 dark:hover:bg-dark-700'
        "
      >
        <!-- Inline rename -->
        <input
          v-if="editingId === conv.id"
          ref="renameInput"
          v-model="editingTitle"
          type="text"
          class="w-full rounded-lg border border-primary-300 bg-white px-3 py-2 text-sm text-gray-900 outline-none focus:ring-1 focus:ring-primary-400 dark:border-primary-600 dark:bg-dark-800 dark:text-white"
          @keydown.enter.prevent="commitRename(conv)"
          @keydown.esc.prevent="cancelRename"
          @blur="commitRename(conv)"
        />

        <!-- Normal row -->
        <template v-else>
          <button
            type="button"
            class="flex min-w-0 flex-1 items-center gap-2 px-3 py-2 text-left text-sm"
            :class="
              conv.id === activeConversationId
                ? 'font-medium text-primary-700 dark:text-primary-300'
                : 'text-gray-700 dark:text-gray-300'
            "
            @click="$emit('select', conv.id)"
          >
            <Icon name="chat" size="sm" class="flex-shrink-0 opacity-60" />
            <span class="truncate">{{ conv.title || t('imageStudio.untitled') }}</span>
          </button>

          <!-- Actions -->
          <div
            class="absolute right-1.5 flex items-center gap-0.5 opacity-0 transition-opacity group-hover:opacity-100"
          >
            <button
              type="button"
              class="rounded-md p-1 text-gray-400 hover:bg-white hover:text-primary-600 dark:hover:bg-dark-600 dark:hover:text-primary-400"
              :title="t('common.rename')"
              @click.stop="startRename(conv)"
            >
              <Icon name="edit" size="xs" />
            </button>
            <button
              type="button"
              class="rounded-md p-1 text-gray-400 hover:bg-white hover:text-red-600 dark:hover:bg-dark-600 dark:hover:text-red-400"
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
</template>

<script setup lang="ts">
import { ref, nextTick } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ImageStudioConversation } from '@/types'
import Icon from '@/components/icons/Icon.vue'

defineProps<{
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
}>()

const { t } = useI18n()

const editingId = ref<number | null>(null)
const editingTitle = ref('')
const renameInput = ref<HTMLInputElement | HTMLInputElement[] | null>(null)

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
