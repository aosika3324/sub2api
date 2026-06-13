<template>
  <div class="auth-shell flex min-h-screen items-center justify-center p-4">
    <div class="relative z-10 w-full max-w-md">
      <div class="mb-7 text-center">
        <div
          class="mb-4 inline-flex h-12 w-12 items-center justify-center overflow-hidden rounded-md border border-[var(--ui-border)] bg-[var(--ui-surface)]"
        >
          <img :src="siteLogo || '/logo.png'" alt="Logo" class="h-full w-full object-contain" />
        </div>
        <h1 class="brand-word mb-2 text-2xl font-semibold text-[var(--ui-text)]">
          {{ siteName }}
        </h1>
        <p class="mx-auto max-w-xs text-sm leading-6 text-[var(--ui-muted)]">
          {{ siteSubtitle }}
        </p>
      </div>

      <div
        class="rounded-lg border border-[var(--ui-border)] bg-[var(--ui-surface)] p-8"
      >
        <slot />
      </div>

      <div class="mt-6 text-center text-sm">
        <slot name="footer" />
      </div>

      <div class="mt-8 text-center text-xs text-[var(--ui-faint)]">
        &copy; {{ currentYear }} {{ siteName }}. All rights reserved.
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores'
import { sanitizeUrl } from '@/utils/url'

const appStore = useAppStore()
const { t } = useI18n()

const siteName = computed(() => appStore.siteName || 'Sub2API')
const siteLogo = computed(() => sanitizeUrl(appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const siteSubtitle = computed(() => appStore.cachedPublicSettings?.site_subtitle || t('home.navSubtitle'))

const currentYear = computed(() => new Date().getFullYear())

onMounted(() => {
  appStore.fetchPublicSettings()
})
</script>

<style scoped>
.auth-shell {
  background: var(--ui-bg);
  letter-spacing: 0;
}

:global(.dark) .auth-shell {
  background: var(--ui-bg);
}

.brand-word {
  font-family: inherit;
  letter-spacing: 0;
}

.auth-shell :deep(.btn-primary) {
  background: var(--ui-text);
  color: var(--ui-bg);
  box-shadow: none;
}

.auth-shell :deep(.btn-primary:hover) {
  opacity: 0.9;
}

.auth-shell :deep(a) {
  color: #176b62;
}

.auth-shell :deep(a:hover) {
  color: #0f4f49;
}

:global(.dark) .auth-shell :deep(.btn-primary) {
  background: var(--ui-text);
  color: var(--ui-bg);
  box-shadow: none;
}

:global(.dark) .auth-shell :deep(.btn-primary:hover) {
  opacity: 0.9;
}

:global(.dark) .auth-shell :deep(a) {
  color: #9be7d8;
}
</style>
