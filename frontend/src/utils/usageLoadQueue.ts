/**
 * Usage request queue that throttles API calls by upstream group.
 *
 * Accounts sharing the same upstream (platform + type + proxy) are placed
 * into one serial queue with a short delay between requests. Different groups
 * run in parallel because they hit different upstreams.
 */

import type { Account } from '@/types'

const GROUP_DELAY_MIN_MS = 1000
const GROUP_DELAY_MAX_MS = 1500

type Task<T> = {
  fn: () => Promise<T>
  resolve: (value: T) => void
  reject: (reason: unknown) => void
}

const queues = new Map<string, Task<unknown>[]>()
const running = new Set<string>()

function buildGroupKey(platform: string, type: string, proxyId: number | null): string {
  return `${platform}:${type}:${proxyId ?? 'direct'}`
}

function accountQueueKey(account: Account): string {
  const queueType =
    account.platform === 'anthropic' && (account.type === 'oauth' || account.type === 'setup-token')
      ? 'claude_code'
      : account.type
  return buildGroupKey(account.platform, queueType, account.proxy_id ?? null)
}

async function drain(groupKey: string) {
  if (running.has(groupKey)) return
  running.add(groupKey)

  const queue = queues.get(groupKey)
  while (queue && queue.length > 0) {
    const task = queue.shift()!
    try {
      const result = await task.fn()
      task.resolve(result)
    } catch (err) {
      task.reject(err)
    }

    if (queue.length > 0) {
      const jitter = GROUP_DELAY_MIN_MS + Math.random() * (GROUP_DELAY_MAX_MS - GROUP_DELAY_MIN_MS)
      await new Promise((resolve) => setTimeout(resolve, jitter))
    }
  }

  running.delete(groupKey)
  queues.delete(groupKey)
}

function enqueueByKey<T>(key: string, fn: () => Promise<T>): Promise<T> {
  return new Promise<T>((resolve, reject) => {
    let queue = queues.get(key)
    if (!queue) {
      queue = []
      queues.set(key, queue)
    }
    queue.push({ fn, resolve, reject } as Task<unknown>)
    drain(key)
  })
}

export function enqueueUsageRequest<T>(
  account: Account,
  fn: () => Promise<T>
): Promise<T>
export function enqueueUsageRequest<T>(
  platform: string,
  type: string,
  proxyId: number | null,
  fn: () => Promise<T>
): Promise<T>
export function enqueueUsageRequest<T>(
  accountOrPlatform: Account | string,
  typeOrFn: string | (() => Promise<T>),
  proxyId?: number | null,
  fn?: () => Promise<T>
): Promise<T> {
  if (typeof accountOrPlatform !== 'string') {
    return enqueueByKey(accountQueueKey(accountOrPlatform), typeOrFn as () => Promise<T>)
  }

  return enqueueByKey(
    buildGroupKey(accountOrPlatform, typeOrFn as string, proxyId ?? null),
    fn as () => Promise<T>
  )
}
