<template>
  <div class="flex h-full flex-col">
    <!-- Header -->
    <div class="flex items-center justify-between px-1 pb-3">
      <h2
        class="text-[11px] font-semibold uppercase tracking-[0.14em] text-gray-400 dark:text-white/40"
      >
        {{ t('imageStudio.conversations') }}
      </h2>
    </div>

    <!-- New conversation (soft neutral pill, not teal) -->
    <button
      type="button"
      class="mb-3 flex w-full items-center justify-center gap-1.5 rounded-xl border border-gray-200 bg-gray-900/[0.04] px-4 py-2.5 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-900/[0.07] disabled:cursor-not-allowed disabled:opacity-60 dark:border-white/10 dark:bg-white/[0.06] dark:text-white dark:hover:bg-white/[0.10]"
      :disabled="creating"
      @click="$emit('create')"
    >
      <Icon name="plus" size="sm" />
      {{ t('imageStudio.newConversation') }}
    </button>

    <!-- Global gallery shortcut -->
    <button
      type="button"
      class="mb-2 flex w-full items-center gap-2 rounded-lg px-3 py-2 text-sm transition-colors"
      :class="
        activeConversationId === null
          ? 'bg-gray-900/[0.06] font-medium text-gray-900 dark:bg-white/[0.06] dark:text-white'
          : 'text-gray-600 hover:bg-gray-900/[0.04] dark:text-white/60 dark:hover:bg-white/[0.04]'
      "
      @click="$emit('select', null)"
    >
      <Icon name="grid" size="sm" class="flex-shrink-0 opacity-50" />
      <span class="truncate">{{ t('imageStudio.allGenerations') }}</span>
    </button>

    <!-- List -->
    <div class="-mx-1 flex-1 space-y-1 overflow-y-auto px-1">
      <!-- Loading -->
      <div v-if="loading && conversations.length === 0" class="space-y-2 pt-1">
        <div
          v-for="i in 4"
          :key="i"
          class="h-10 animate-pulse rounded-lg bg-gray-900/[0.05] dark:bg-white/[0.05]"
        ></div>
      </div>

      <!-- Empty -->
      <p
        v-else-if="conversations.length === 0"
        class="px-3 py-6 text-center text-xs text-gray-400 dark:text-white/35"
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
            ? 'bg-gray-900/[0.06] dark:bg-white/[0.06]'
            : 'hover:bg-gray-900/[0.04] dark:hover:bg-white/[0.04]'
        "
      >
        <!-- Inline rename -->
        <input
          v-if="editingId === conv.id"
          ref="renameInput"
          v-model="editingTitle"
          type="text"
          class="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 outline-none focus:ring-1 focus:ring-gray-400 dark:border-white/20 dark:bg-white/[0.04] dark:text-white dark:focus:ring-white/30"
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
                ? 'font-medium text-gray-900 dark:text-white'
                : 'text-gray-700 dark:text-white/60'
            "
            @click="$emit('select', conv.id)"
          >
            <Icon name="chat" size="sm" class="flex-shrink-0 opacity-40" />
            <span class="truncate">{{ conv.title || t('imageStudio.untitled') }}</span>
          </button>

          <!-- Actions -->
          <div
            class="absolute right-1.5 flex items-center gap-0.5 opacity-0 transition-opacity group-hover:opacity-100"
          >
            <button
              type="button"
              class="rounded-md p-1 text-gray-400 hover:bg-gray-900/[0.06] hover:text-gray-700 dark:text-white/40 dark:hover:bg-white/[0.10] dark:hover:text-white"
              :title="t('common.rename')"
              @click.stop="startRename(conv)"
            >
              <Icon name="edit" size="xs" />
            </button>
            <button
              type="button"
              class="rounded-md p-1 text-gray-400 hover:bg-red-50 hover:text-red-600 dark:text-white/40 dark:hover:bg-red-500/15 dark:hover:text-red-400"
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
