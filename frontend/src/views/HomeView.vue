<template>
  <div v-if="homeContent" class="min-h-screen">
    <iframe
      v-if="isHomeContentUrl"
      :src="homeContent.trim()"
      class="h-screen w-full border-0"
      allowfullscreen
    ></iframe>
    <div v-else v-html="homeContent"></div>
  </div>

  <div v-else class="home-page min-h-screen bg-[var(--ui-bg)] text-[var(--ui-text)]">
    <header class="border-b border-[var(--ui-border)]">
      <nav class="mx-auto flex h-16 max-w-6xl items-center justify-between px-5">
        <div class="flex items-center gap-3">
          <div class="flex h-8 w-8 items-center justify-center overflow-hidden rounded-md">
            <img :src="siteLogo || '/logo.png'" alt="Logo" class="h-full w-full object-contain" />
          </div>
          <span class="text-sm font-semibold">{{ siteName }}</span>
        </div>

        <div class="flex items-center gap-2">
          <LocaleSwitcher />
          <a
            v-if="docUrl"
            :href="docUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="quiet-icon"
            :title="t('home.viewDocs')"
          >
            <Icon name="book" size="sm" />
          </a>
          <button
            class="quiet-icon"
            :title="isDark ? t('home.switchToLight') : t('home.switchToDark')"
            @click="toggleTheme"
          >
            <Icon v-if="isDark" name="sun" size="sm" />
            <Icon v-else name="moon" size="sm" />
          </button>
          <router-link
            :to="isAuthenticated ? dashboardPath : '/login'"
            class="inline-flex h-9 items-center rounded-md bg-[var(--ui-text)] px-3 text-sm font-medium text-[var(--ui-bg)]"
          >
            {{ isAuthenticated ? t('home.dashboard') : t('home.login') }}
          </router-link>
        </div>
      </nav>
    </header>

    <main>
      <section class="hero-section mx-auto max-w-6xl px-5 pb-24 pt-20 md:pb-32 md:pt-28">
        <div class="relative z-10 max-w-4xl">
          <p class="mb-6 text-sm font-medium text-[var(--ui-muted)]">{{ t('home.heroEyebrow') }}</p>
          <h1 class="hero-title max-w-4xl text-4xl font-semibold leading-[1.06] sm:text-5xl md:text-7xl">
            {{ t('home.heroTitle') }}
          </h1>
          <p class="mt-8 max-w-2xl text-lg leading-8 text-[var(--ui-muted)] md:text-xl">
            {{ t('home.heroDescription') }}
          </p>

          <div class="mt-9 flex flex-col gap-3 sm:flex-row">
            <router-link
              :to="isAuthenticated ? dashboardPath : '/login'"
              class="cta-btn inline-flex h-11 items-center justify-center rounded-md bg-[var(--ui-text)] px-5 text-sm font-semibold text-[var(--ui-bg)]"
            >
              {{ isAuthenticated ? t('home.goToDashboard') : t('home.getStarted') }}
              <Icon name="arrowRight" size="sm" class="ml-2" :stroke-width="2" />
            </router-link>
            <a
              v-if="docUrl"
              :href="docUrl"
              target="_blank"
              rel="noopener noreferrer"
              class="inline-flex h-11 items-center justify-center rounded-md border border-[var(--ui-border)] bg-[var(--ui-surface)] px-5 text-sm font-semibold text-[var(--ui-text)]"
            >
              {{ t('home.docs') }}
            </a>
          </div>
        </div>

        <div class="relative z-10 mt-20 grid border-y border-[var(--ui-border)] md:grid-cols-3">
          <div
            v-for="(item, i) in valueRows"
            :key="item.title"
            v-reveal
            class="value-cell reveal"
            :style="{ transitionDelay: i * 70 + 'ms' }"
          >
            <p class="text-sm font-semibold">{{ item.title }}</p>
            <p class="mt-3 text-sm leading-6 text-[var(--ui-muted)]">{{ item.description }}</p>
          </div>
        </div>
      </section>

      <section class="border-t border-[var(--ui-border)] bg-[var(--ui-surface)]">
        <div class="mx-auto grid max-w-6xl gap-12 px-5 py-20 lg:grid-cols-[0.85fr_1.15fr]">
          <div>
            <p class="mb-4 text-sm font-medium text-[var(--ui-muted)]">{{ t('home.product.eyebrow') }}</p>
            <h2 class="text-3xl font-semibold md:text-5xl">
              {{ t('home.product.title') }}
            </h2>
            <p class="mt-6 max-w-md text-base leading-7 text-[var(--ui-muted)]">
              {{ t('home.product.description') }}
            </p>
          </div>

          <div class="space-y-3">
            <article
              v-for="(feature, i) in featureCards"
              :key="feature.title"
              v-reveal
              class="product-row reveal"
              :style="{ transitionDelay: i * 70 + 'ms' }"
            >
              <div class="product-icon">
                <Icon :name="feature.icon" size="sm" />
              </div>
              <div>
                <h3 class="text-base font-semibold">{{ feature.title }}</h3>
                <p class="mt-2 text-sm leading-6 text-[var(--ui-muted)]">{{ feature.description }}</p>
              </div>
            </article>
          </div>
        </div>
      </section>

      <section class="mx-auto max-w-6xl px-5 py-20">
        <div class="mb-10 flex flex-col justify-between gap-6 md:flex-row md:items-end">
          <div>
            <p class="mb-4 text-sm font-medium text-[var(--ui-muted)]">{{ t('home.workflow.eyebrow') }}</p>
            <h2 class="text-3xl font-semibold md:text-5xl">
              {{ t('home.workflow.title') }}
            </h2>
          </div>
          <p class="max-w-md text-sm leading-6 text-[var(--ui-muted)]">
            {{ t('home.workflow.description') }}
          </p>
        </div>

        <div class="grid gap-4 lg:grid-cols-2">
          <div ref="codePanelRef" v-reveal class="work-panel reveal">
            <div class="mb-6 flex items-center justify-between">
              <span class="text-sm font-semibold">{{ t('home.workflow.codeTitle') }}</span>
              <Icon name="terminal" size="sm" class="text-[var(--ui-muted)]" />
            </div>
            <pre class="work-code"><code>{{ typedCode }}<span class="term-cursor" aria-hidden="true"></span></code></pre>
          </div>
          <div v-reveal class="work-panel reveal" :style="{ transitionDelay: '90ms' }">
            <div class="mb-6 flex items-center justify-between">
              <span class="text-sm font-semibold">{{ t('home.workflow.imageTitle') }}</span>
              <Icon name="sparkles" size="sm" class="text-[var(--ui-muted)]" />
            </div>
            <div class="space-y-4">
              <div v-for="line in promptLines" :key="line.label" class="prompt-row">
                <span>{{ line.label }}</span>
                <p>{{ line.value }}</p>
              </div>
            </div>
          </div>
        </div>
      </section>

      <section class="border-y border-[var(--ui-border)] bg-[var(--ui-surface)]">
        <div class="mx-auto max-w-6xl px-5 py-16">
          <div class="grid gap-4 md:grid-cols-5">
            <div
              v-for="(provider, i) in providers"
              :key="provider.name"
              v-reveal
              class="provider-row reveal"
              :style="{ transitionDelay: i * 60 + 'ms' }"
            >
              <span class="provider-initial">{{ provider.initial }}</span>
              <div>
                <p class="text-sm font-semibold">{{ provider.name }}</p>
                <p class="text-xs text-[var(--ui-muted)]">{{ provider.status }}</p>
              </div>
            </div>
          </div>
        </div>
      </section>

      <section class="mx-auto max-w-6xl px-5 py-20">
        <div class="cta-simple">
          <h2 class="max-w-2xl text-3xl font-semibold md:text-5xl">
            {{ t('home.cta.title') }}
          </h2>
          <p class="mt-5 max-w-xl text-base leading-7 text-[var(--ui-muted)]">
            {{ t('home.cta.description') }}
          </p>
          <router-link
            :to="isAuthenticated ? dashboardPath : '/register'"
            class="cta-btn mt-8 inline-flex h-11 items-center rounded-md bg-[var(--ui-text)] px-5 text-sm font-semibold text-[var(--ui-bg)]"
          >
            {{ isAuthenticated ? t('home.goToDashboard') : t('home.cta.button') }}
            <Icon name="arrowRight" size="sm" class="ml-2" :stroke-width="2" />
          </router-link>
        </div>
      </section>
    </main>

    <footer class="border-t border-[var(--ui-border)] px-5 py-8">
      <div class="mx-auto flex max-w-6xl flex-col justify-between gap-3 text-sm text-[var(--ui-muted)] sm:flex-row">
        <p>&copy; {{ currentYear }} {{ siteName }}. {{ t('home.footer.allRightsReserved') }}</p>
        <a v-if="docUrl" :href="docUrl" target="_blank" rel="noopener noreferrer">
          {{ t('home.docs') }}
        </a>
      </div>
    </footer>

    <WechatServiceButton />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, type Directive } from 'vue'
import { useI18n } from 'vue-i18n'
import { useIntersectionObserver } from '@vueuse/core'
import { useAuthStore, useAppStore } from '@/stores'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import Icon from '@/components/icons/Icon.vue'
import WechatServiceButton from '@/components/common/WechatServiceButton.vue'
import { sanitizeUrl } from '@/utils/url'

const { t } = useI18n()
const authStore = useAuthStore()
const appStore = useAppStore()

const prefersReducedMotion =
  typeof window !== 'undefined' &&
  window.matchMedia &&
  window.matchMedia('(prefers-reduced-motion: reduce)').matches

type IconName = InstanceType<typeof Icon>['$props']['name']

const siteName = computed(() => appStore.cachedPublicSettings?.site_name || appStore.siteName || 'Sub2API')
const siteLogo = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.site_logo || appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const docUrl = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.doc_url || appStore.docUrl || ''))
const homeContent = computed(() => appStore.cachedPublicSettings?.home_content || '')
const isHomeContentUrl = computed(() => {
  const content = homeContent.value.trim()
  return content.startsWith('http://') || content.startsWith('https://')
})

const isDark = ref(document.documentElement.classList.contains('dark'))
const isAuthenticated = computed(() => authStore.isAuthenticated)
const isAdmin = computed(() => authStore.isAdmin)
const dashboardPath = computed(() => isAdmin.value ? '/admin/dashboard' : '/dashboard')
const currentYear = computed(() => new Date().getFullYear())

const valueRows = computed(() => [
  { title: t('home.values.oneKey.title'), description: t('home.values.oneKey.description') },
  { title: t('home.values.routing.title'), description: t('home.values.routing.description') },
  { title: t('home.values.cost.title'), description: t('home.values.cost.description') }
])

const featureCards = computed<Array<{ icon: IconName; title: string; description: string }>>(() => [
  { icon: 'terminal', title: t('home.features.unifiedGateway'), description: t('home.features.unifiedGatewayDesc') },
  { icon: 'sparkles', title: t('home.features.multiAccount'), description: t('home.features.multiAccountDesc') },
  { icon: 'chart', title: t('home.features.balanceQuota'), description: t('home.features.balanceQuotaDesc') }
])

const providers = computed(() => [
  { name: t('home.providers.claude'), initial: 'C', status: t('home.providers.supported') },
  { name: 'GPT', initial: 'G', status: t('home.providers.supported') },
  { name: t('home.providers.gemini'), initial: 'G', status: t('home.providers.supported') },
  { name: t('home.providers.antigravity'), initial: 'A', status: t('home.providers.supported') },
  { name: t('home.providers.more'), initial: '+', status: t('home.providers.soon') }
])

const codeSample = [
  'const client = new Sub2API({ key: process.env.API_KEY })',
  '',
  'await client.messages.create({',
  '  model: "claude-or-gpt",',
  '  intent: "refactor-vue-component",',
  '  input: "simplify this billing table"',
  '})'
].join('\n')

// Typewriter: types codeSample once the panel scrolls into view (or shows it
// immediately when reduced motion is preferred).
const codePanelRef = ref<HTMLElement | null>(null)
const typedCode = ref(prefersReducedMotion ? codeSample : '')
let typeTimer: ReturnType<typeof setInterval> | null = null
let typingStarted = false

function startTyping() {
  if (typingStarted || prefersReducedMotion) return
  typingStarted = true
  let i = 0
  typeTimer = setInterval(() => {
    i += 1
    typedCode.value = codeSample.slice(0, i)
    if (i >= codeSample.length && typeTimer) {
      clearInterval(typeTimer)
      typeTimer = null
    }
  }, 18)
}

if (!prefersReducedMotion) {
  const { stop } = useIntersectionObserver(
    codePanelRef,
    ([entry]) => {
      if (entry?.isIntersecting) {
        startTyping()
        stop()
      }
    },
    { threshold: 0.3 }
  )
}

// v-reveal: fade+slide a block in when it enters the viewport (once). Honors
// reduced-motion by revealing immediately without observing.
const revealObservers = new WeakMap<Element, () => void>()
const vReveal: Directive<HTMLElement> = {
  mounted(el) {
    if (prefersReducedMotion) {
      el.classList.add('is-visible')
      return
    }
    const { stop } = useIntersectionObserver(
      el,
      ([entry]) => {
        if (entry?.isIntersecting) {
          el.classList.add('is-visible')
          stop()
          revealObservers.delete(el)
        }
      },
      { threshold: 0.15 }
    )
    revealObservers.set(el, stop)
  },
  unmounted(el) {
    revealObservers.get(el)?.()
    revealObservers.delete(el)
  }
}

onBeforeUnmount(() => {
  if (typeTimer) clearInterval(typeTimer)
})

const promptLines = computed(() => [
  { label: t('home.workflow.prompt'), value: t('home.workflow.promptValue') },
  { label: t('home.workflow.model'), value: t('home.workflow.modelValue') },
  { label: t('home.workflow.policy'), value: t('home.workflow.policyValue') }
])

function toggleTheme() {
  isDark.value = !isDark.value
  document.documentElement.classList.toggle('dark', isDark.value)
  localStorage.setItem('theme', isDark.value ? 'dark' : 'light')
}

function initTheme() {
  const savedTheme = localStorage.getItem('theme')
  if (
    savedTheme === 'dark' ||
    (!savedTheme && window.matchMedia('(prefers-color-scheme: dark)').matches)
  ) {
    isDark.value = true
    document.documentElement.classList.add('dark')
  }
}

onMounted(() => {
  initTheme()
  authStore.checkAuth()
  if (!appStore.publicSettingsLoaded) {
    appStore.fetchPublicSettings()
  }
})
</script>

<style scoped>
.home-page {
  letter-spacing: 0;
  --home-coral: #d97757;
}

:global(.dark) .home-page {
  --home-coral: #e8896b;
}

/* ---- Hero headline animated coral sheen ---- */
.hero-title {
  color: var(--ui-text);
}

@supports ((-webkit-background-clip: text) or (background-clip: text)) {
  .hero-title {
    background: linear-gradient(
      110deg,
      var(--ui-text) 0%,
      var(--ui-text) 55%,
      var(--home-coral) 80%,
      var(--ui-accent) 100%
    );
    background-size: 200% 100%;
    -webkit-background-clip: text;
    background-clip: text;
    -webkit-text-fill-color: transparent;
    animation: heroSheen 8s ease-in-out infinite alternate;
  }
}

@keyframes heroSheen {
  from {
    background-position: 0% 50%;
  }
  to {
    background-position: 100% 50%;
  }
}

/* ---- Drifting dot-grid texture behind the hero ---- */
.hero-section {
  position: relative;
  overflow: hidden;
}

.hero-section::before {
  content: '';
  position: absolute;
  inset: 0;
  z-index: 0;
  background-image: radial-gradient(var(--ui-border) 1px, transparent 1px);
  background-size: 22px 22px;
  -webkit-mask-image: radial-gradient(ellipse 80% 60% at 30% 0%, #000 0%, transparent 70%);
  mask-image: radial-gradient(ellipse 80% 60% at 30% 0%, #000 0%, transparent 70%);
  opacity: 0.6;
  animation: gridDrift 40s linear infinite;
  pointer-events: none;
}

@keyframes gridDrift {
  to {
    background-position: 22px 22px;
  }
}

/* ---- Terminal cursor ---- */
.term-cursor {
  display: inline-block;
  width: 7px;
  height: 1.05em;
  margin-left: 2px;
  vertical-align: text-bottom;
  background: #f5f2ea;
  animation: termBlink 1.06s steps(1) infinite;
}

@keyframes termBlink {
  0%,
  50% {
    opacity: 1;
  }
  50.01%,
  100% {
    opacity: 0;
  }
}

/* ---- Scroll-triggered reveal ---- */
.reveal {
  opacity: 0;
  transform: translateY(16px);
  transition: opacity 0.55s ease, transform 0.55s ease;
}

.reveal.is-visible {
  opacity: 1;
  transform: none;
}

/* ---- CTA arrow nudge + coral-tinted shadow ---- */
.cta-btn {
  transition: transform 0.2s ease, box-shadow 0.2s ease;
}

.cta-btn:hover {
  box-shadow: 0 6px 18px -8px var(--home-coral);
}

.cta-btn :deep(svg) {
  transition: transform 0.2s ease;
}

.cta-btn:hover :deep(svg) {
  transform: translateX(3px);
}

/* ---- Reduced-motion guard ---- */
@media (prefers-reduced-motion: reduce) {
  .hero-title {
    animation: none;
    background: none;
    -webkit-text-fill-color: currentColor;
  }
  .hero-section::before {
    animation: none;
  }
  .term-cursor {
    opacity: 1;
    animation: none;
  }
  .reveal {
    opacity: 1;
    transform: none;
    transition: none;
  }
  .cta-btn,
  .cta-btn :deep(svg),
  .product-row,
  .work-panel,
  .provider-row {
    transition: none;
  }
}

.quiet-icon {
  display: inline-flex;
  height: 36px;
  width: 36px;
  align-items: center;
  justify-content: center;
  border-radius: 6px;
  color: var(--ui-muted);
}

.quiet-icon:hover {
  background: var(--ui-surface-muted);
  color: var(--ui-text);
}

.value-cell {
  padding: 24px 0;
}

.value-cell + .value-cell {
  border-top: 1px solid var(--ui-border);
}

.product-row {
  display: grid;
  grid-template-columns: 36px minmax(0, 1fr);
  gap: 16px;
  border: 1px solid var(--ui-border);
  border-radius: 8px;
  background: var(--ui-surface);
  padding: 18px;
  transition: transform 0.2s ease, box-shadow 0.2s ease, border-color 0.2s ease;
}

.product-row:hover {
  transform: translateY(-2px);
  border-color: var(--ui-border-strong);
  box-shadow: 0 8px 24px -12px rgba(0, 0, 0, 0.18);
}

:global(.dark) .product-row:hover {
  box-shadow: 0 8px 28px -10px rgba(0, 0, 0, 0.6);
}

.product-icon {
  display: flex;
  height: 36px;
  width: 36px;
  align-items: center;
  justify-content: center;
  border-radius: 6px;
  border: 1px solid var(--ui-border);
  color: var(--ui-muted);
  transition: color 0.2s ease, border-color 0.2s ease;
}

.product-row:hover .product-icon {
  border-color: var(--ui-accent);
  color: var(--ui-accent);
}

.work-panel {
  min-width: 0;
  min-height: 360px;
  border: 1px solid var(--ui-border);
  border-radius: 8px;
  background: var(--ui-surface);
  padding: 20px;
  transition: transform 0.2s ease, box-shadow 0.2s ease, border-color 0.2s ease;
}

.work-panel:hover {
  transform: translateY(-2px);
  border-color: var(--ui-border-strong);
  box-shadow: 0 8px 24px -12px rgba(0, 0, 0, 0.18);
}

:global(.dark) .work-panel:hover {
  box-shadow: 0 8px 28px -10px rgba(0, 0, 0, 0.6);
}

.work-code {
  max-width: 100%;
  min-height: 264px;
  overflow: auto;
  border-radius: 6px;
  background: #171613;
  color: #f5f2ea;
  padding: 18px;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 13px;
  line-height: 1.8;
}

.work-code code {
  display: block;
  min-width: max-content;
}

.prompt-row {
  border-bottom: 1px solid var(--ui-border);
  padding-bottom: 14px;
}

.prompt-row span {
  display: block;
  margin-bottom: 6px;
  font-size: 12px;
  font-weight: 600;
  color: var(--ui-muted);
}

.prompt-row p {
  font-size: 15px;
  line-height: 1.8;
}

.provider-row {
  display: flex;
  align-items: center;
  gap: 12px;
  border: 1px solid var(--ui-border);
  border-radius: 8px;
  padding: 14px;
  transition: transform 0.2s ease, box-shadow 0.2s ease, border-color 0.2s ease;
}

.provider-row:hover {
  transform: translateY(-2px);
  border-color: var(--ui-border-strong);
  box-shadow: 0 8px 24px -12px rgba(0, 0, 0, 0.18);
}

:global(.dark) .provider-row:hover {
  box-shadow: 0 8px 28px -10px rgba(0, 0, 0, 0.6);
}

.provider-initial {
  display: inline-flex;
  height: 32px;
  width: 32px;
  align-items: center;
  justify-content: center;
  border-radius: 6px;
  background: var(--ui-text);
  color: var(--ui-bg);
  font-weight: 700;
  font-size: 13px;
}

.cta-simple {
  border: 1px solid var(--ui-border);
  border-radius: 8px;
  background: var(--ui-surface);
  padding: 32px;
}

@media (min-width: 768px) {
  .value-cell {
    padding: 24px;
  }

  .value-cell + .value-cell {
    border-top: 0;
    border-left: 1px solid var(--ui-border);
  }
}
</style>
