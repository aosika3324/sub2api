import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useImageStudioStore } from '@/stores/imageStudio'
import { useAuthStore } from '@/stores/auth'

// ==================== Mock @/api/imageStudio ====================

const mockGenerate = vi.fn()
const mockListConversations = vi.fn()
const mockCreateConversation = vi.fn()
const mockRenameConversation = vi.fn()
const mockDeleteConversation = vi.fn()
const mockListConversationGenerations = vi.fn()
const mockListGenerations = vi.fn()
const mockDeleteGeneration = vi.fn()

vi.mock('@/api/imageStudio', () => ({
  default: {
    generate: (...args: any[]) => mockGenerate(...args),
    listConversations: (...args: any[]) => mockListConversations(...args),
    createConversation: (...args: any[]) => mockCreateConversation(...args),
    renameConversation: (...args: any[]) => mockRenameConversation(...args),
    deleteConversation: (...args: any[]) => mockDeleteConversation(...args),
    listConversationGenerations: (...args: any[]) => mockListConversationGenerations(...args),
    listGenerations: (...args: any[]) => mockListGenerations(...args),
    deleteGeneration: (...args: any[]) => mockDeleteGeneration(...args),
  },
}))

// ==================== Fixtures ====================

const fakeConversation = {
  id: 1,
  title: 'My first session',
  created_at: '2026-06-07T00:00:00Z',
  updated_at: '2026-06-07T00:00:00Z',
}

const fakeConversation2 = {
  id: 2,
  title: 'Second session',
  created_at: '2026-06-07T01:00:00Z',
  updated_at: '2026-06-07T01:00:00Z',
}

const fakeGenerateReq = {
  group_id: 10,
  prompt: 'A sunset over the mountains',
  model: 'gpt-image-2',
  size: '1K',
  quality: 'high',
  n: 1,
}

const fakeGenerateResp = {
  generation_id: 42,
  conversation_id: 1,
  images: ['https://cdn.example.com/img1.png'],
  cost: 0.04,
  balance: 9.96,
}

const fakeGeneration = {
  id: 42,
  conversation_id: 1,
  group_id: 10,
  prompt: 'A sunset over the mountains',
  model: 'gpt-image-2',
  size: '1K',
  quality: 'high',
  n: 1,
  image_count: 1,
  status: 'completed',
  cost: 0.04,
  created_at: expect.any(String),
  images: ['https://cdn.example.com/img1.png'],
}

// ==================== Tests ====================

describe('useImageStudioStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  // --- generate success ---

  describe('generate', () => {
    it('成功生成: 新生成被前置到 generations, 余额更新到 authStore', async () => {
      mockGenerate.mockResolvedValue(fakeGenerateResp)

      // Seed auth store with a user who has a known balance
      const authStore = useAuthStore()
      authStore.user = {
        id: 1,
        username: 'tester',
        email: 'tester@example.com',
        role: 'user',
        balance: 10,
        concurrency: 5,
        status: 'active',
        allowed_groups: null,
        balance_notify_enabled: false,
        balance_notify_threshold: null,
        balance_notify_extra_emails: [],
        created_at: '2026-01-01T00:00:00Z',
        updated_at: '2026-01-01T00:00:00Z',
      }

      const store = useImageStudioStore()
      await store.generate(fakeGenerateReq)

      // Generation prepended
      expect(store.generations).toHaveLength(1)
      expect(store.generations[0]).toMatchObject(fakeGeneration)

      // Balance updated
      expect(authStore.user.balance).toBe(9.96)

      // No error, not generating
      expect(store.error).toBeNull()
      expect(store.generating).toBe(false)
    })

    it('generate 成功后前置: 新生成在旧记录之前', async () => {
      mockGenerate.mockResolvedValue(fakeGenerateResp)

      const store = useImageStudioStore()
      // Seed with an existing generation
      store.generations = [
        {
          id: 1,
          conversation_id: 1,
          group_id: 10,
          prompt: 'old prompt',
          model: 'gpt-image-2',
          size: '1K',
          quality: 'high',
          n: 1,
          image_count: 1,
          status: 'completed',
          cost: 0.04,
          created_at: '2026-06-06T00:00:00Z',
        },
      ]

      await store.generate(fakeGenerateReq)

      expect(store.generations).toHaveLength(2)
      expect(store.generations[0].id).toBe(42) // new one first
    })

    it('generate 失败: error 被设置, generations 不变', async () => {
      const err = new Error('rate limited')
      mockGenerate.mockRejectedValue(err)

      const store = useImageStudioStore()
      const initialGenerations = [...store.generations]

      await expect(store.generate(fakeGenerateReq)).rejects.toThrow('rate limited')

      expect(store.error).toBe(err)
      expect(store.generations).toEqual(initialGenerations)
      expect(store.generating).toBe(false)
    })

    it('generate 失败时不更新余额', async () => {
      mockGenerate.mockRejectedValue(new Error('fail'))

      const authStore = useAuthStore()
      authStore.user = {
        id: 1,
        username: 'tester',
        email: 'tester@example.com',
        role: 'user',
        balance: 10,
        concurrency: 5,
        status: 'active',
        allowed_groups: null,
        balance_notify_enabled: false,
        balance_notify_threshold: null,
        balance_notify_extra_emails: [],
        created_at: '2026-01-01T00:00:00Z',
        updated_at: '2026-01-01T00:00:00Z',
      }

      const store = useImageStudioStore()
      await expect(store.generate(fakeGenerateReq)).rejects.toThrow()

      expect(authStore.user.balance).toBe(10) // unchanged
    })
  })

  // --- loadConversations ---

  describe('loadConversations', () => {
    it('成功加载: conversations 被填充', async () => {
      mockListConversations.mockResolvedValue({
        items: [fakeConversation, fakeConversation2],
        total: 2,
        page: 1,
        page_size: 50,
        pages: 1,
      })

      const store = useImageStudioStore()
      await store.loadConversations()

      expect(store.conversations).toHaveLength(2)
      expect(store.conversations[0]).toEqual(fakeConversation)
      expect(store.loading).toBe(false)
      expect(store.error).toBeNull()
    })

    it('加载失败: error 被设置并重新抛出', async () => {
      const err = new Error('network error')
      mockListConversations.mockRejectedValue(err)

      const store = useImageStudioStore()
      await expect(store.loadConversations()).rejects.toThrow('network error')

      expect(store.error).toBe(err)
      expect(store.loading).toBe(false)
    })
  })

  // --- createConversation ---

  describe('createConversation', () => {
    it('新建对话被前置到 conversations', async () => {
      mockCreateConversation.mockResolvedValue(fakeConversation)

      const store = useImageStudioStore()
      store.conversations = [fakeConversation2]

      const result = await store.createConversation('My first session')

      expect(result).toEqual(fakeConversation)
      expect(store.conversations).toHaveLength(2)
      expect(store.conversations[0]).toEqual(fakeConversation)
    })
  })

  // --- renameConversation ---

  describe('renameConversation', () => {
    it('重命名后本地列表同步更新', async () => {
      const updated = { ...fakeConversation, title: 'Renamed' }
      mockRenameConversation.mockResolvedValue(updated)

      const store = useImageStudioStore()
      store.conversations = [fakeConversation]

      await store.renameConversation(1, 'Renamed')

      expect(store.conversations[0].title).toBe('Renamed')
    })
  })

  // --- deleteConversation ---

  describe('deleteConversation', () => {
    it('删除后从 conversations 中移除', async () => {
      mockDeleteConversation.mockResolvedValue(undefined)

      const store = useImageStudioStore()
      store.conversations = [fakeConversation, fakeConversation2]

      await store.deleteConversation(1)

      expect(store.conversations).toHaveLength(1)
      expect(store.conversations[0].id).toBe(2)
    })

    it('删除激活的对话后清除 activeConversationId', async () => {
      mockDeleteConversation.mockResolvedValue(undefined)

      const store = useImageStudioStore()
      store.conversations = [fakeConversation]
      store.activeConversationId = 1

      await store.deleteConversation(1)

      expect(store.activeConversationId).toBeNull()
    })
  })

  // --- selectConversation ---

  describe('selectConversation', () => {
    it('设置 activeConversationId', () => {
      const store = useImageStudioStore()
      store.selectConversation(5)
      expect(store.activeConversationId).toBe(5)
    })

    it('传 null 清除选中', () => {
      const store = useImageStudioStore()
      store.activeConversationId = 5
      store.selectConversation(null)
      expect(store.activeConversationId).toBeNull()
    })
  })

  // --- loadGenerations ---

  describe('loadGenerations', () => {
    it('不传 conversationId: 调用全局列表接口', async () => {
      mockListGenerations.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20, pages: 0 })

      const store = useImageStudioStore()
      await store.loadGenerations()

      expect(mockListGenerations).toHaveBeenCalledTimes(1)
      expect(mockListConversationGenerations).not.toHaveBeenCalled()
    })

    it('传入 conversationId: 调用对话级别接口', async () => {
      mockListConversationGenerations.mockResolvedValue({
        items: [],
        total: 0,
        page: 1,
        page_size: 20,
        pages: 0,
      })

      const store = useImageStudioStore()
      await store.loadGenerations(1)

      expect(mockListConversationGenerations).toHaveBeenCalledWith(1)
      expect(mockListGenerations).not.toHaveBeenCalled()
    })

    it('hasLoadedGenerations: 初始为 false, 首次加载后变 true (用于门控骨架屏只在首屏显示)', async () => {
      mockListGenerations.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20, pages: 0 })

      const store = useImageStudioStore()
      expect(store.hasLoadedGenerations).toBe(false)

      await store.loadGenerations()

      expect(store.hasLoadedGenerations).toBe(true)
    })

    it('hasLoadedGenerations: 加载失败也置为 true (避免永久骨架屏)', async () => {
      mockListGenerations.mockRejectedValue(new Error('boom'))

      const store = useImageStudioStore()
      await expect(store.loadGenerations()).rejects.toThrow('boom')

      expect(store.hasLoadedGenerations).toBe(true)
    })
  })

  // --- resetGenerations ---

  describe('resetGenerations', () => {
    it('清空 generations 并标记已加载 (新建空会话即时显示空态, 不触发骨架屏/网络请求)', () => {
      const store = useImageStudioStore()
      store.generations = [
        {
          id: 1,
          conversation_id: 1,
          group_id: 10,
          prompt: 'old',
          model: 'gpt-image-2',
          size: '1K',
          quality: 'high',
          n: 1,
          image_count: 1,
          status: 'completed',
          cost: 0.04,
          created_at: '2026-06-07T00:00:00Z',
        },
      ]

      store.resetGenerations()

      expect(store.generations).toHaveLength(0)
      expect(store.hasLoadedGenerations).toBe(true)
      // No network call involved
      expect(mockListGenerations).not.toHaveBeenCalled()
      expect(mockListConversationGenerations).not.toHaveBeenCalled()
    })
  })

  // --- deleteGeneration ---

  describe('deleteGeneration', () => {
    it('删除后从 generations 中移除', async () => {
      mockDeleteGeneration.mockResolvedValue(undefined)

      const store = useImageStudioStore()
      store.generations = [
        {
          id: 42,
          conversation_id: 1,
          group_id: 10,
          prompt: 'test',
          model: 'gpt-image-2',
          size: '1K',
          quality: 'high',
          n: 1,
          image_count: 1,
          status: 'completed',
          cost: 0.04,
          created_at: '2026-06-07T00:00:00Z',
        },
      ]

      await store.deleteGeneration(42)

      expect(store.generations).toHaveLength(0)
      expect(mockDeleteGeneration).toHaveBeenCalledWith(42)
    })
  })
})
