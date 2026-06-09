import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useImageStudioStore } from '@/stores/imageStudio'

const mockGenerate = vi.fn()

vi.mock('@/api/imageStudio', () => ({
  default: {
    generate: (...args: unknown[]) => mockGenerate(...args),
    listConversations: vi.fn(),
    createConversation: vi.fn(),
    renameConversation: vi.fn(),
    deleteConversation: vi.fn(),
    listConversationGenerations: vi.fn(),
    listGenerations: vi.fn(),
    getGeneration: vi.fn(),
    deleteGeneration: vi.fn(),
    clearHistory: vi.fn(),
  },
}))

describe('useImageStudioStore pending generations', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('inserts an accepted async generation as pending and pollable', async () => {
    mockGenerate.mockResolvedValue({
      generation_id: 43,
      conversation_id: 7,
      images: [],
      input_images: ['/api/v1/user/image-studio/input-assets/43/0'],
      status: 'pending',
      cost: 0,
      balance: 10,
    })

    const store = useImageStudioStore()
    await store.generate({
      group_id: 10,
      prompt: 'edit this image',
      model: 'gpt-image-2',
      size: '1024x1024',
      quality: 'auto',
      n: 1,
    })

    expect(store.generations).toHaveLength(1)
    expect(store.generations[0]).toMatchObject({
      id: 43,
      conversation_id: 7,
      status: 'pending',
      image_count: 0,
      images: [],
      input_images: ['/api/v1/user/image-studio/input-assets/43/0'],
    })
  })
})
