// SPDX-License-Identifier: Apache-2.0

import { describe, it, expect } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import { useMetricsHistory } from './useMetricsHistory'
import type { Metrics } from '../types/metrics'

const createMockMetrics = (timestamp: string): Metrics => ({
  timestamp,
  jobs_active: 5,
  jobs_completed: 100,
  jobs_failed: 2,
  jobs_pending: 10,
  jobs_cancelled: 1,
  queue_length: 15,
  http_requests: 1000,
  http_errors: 5,
  avg_response_time: 50,
  memory_usage: 536870912,
  cpu_usage: 25,
  goroutines: 50,
  active_connections: 3,
  websocket_clients: 2,
  provider_stats: {},
  recent_jobs: [],
  system_health: 'healthy',
  alerts: [],
  uptime_seconds: 3600,
})

describe('useMetricsHistory', () => {
  it('initializes with empty history', () => {
    const { result } = renderHook(() => useMetricsHistory())
    expect(result.current.history).toEqual([])
  })

  it('adds metrics to history', () => {
    const { result } = renderHook(() => useMetricsHistory())
    const metrics = createMockMetrics('2026-01-20T12:00:00Z')

    act(() => {
      result.current.addMetrics(metrics)
    })

    expect(result.current.history).toHaveLength(1)
    expect(result.current.history[0]).toEqual(metrics)
  })

  it('limits history to maxPoints', () => {
    const { result } = renderHook(() => useMetricsHistory(5))

    // Add 10 metrics
    act(() => {
      for (let i = 0; i < 10; i++) {
        result.current.addMetrics(createMockMetrics(`2026-01-20T12:${i}:00Z`))
      }
    })

    // Should only keep last 5
    expect(result.current.history).toHaveLength(5)
    expect(result.current.history[0].timestamp).toBe('2026-01-20T12:5:00Z')
    expect(result.current.history[4].timestamp).toBe('2026-01-20T12:9:00Z')
  })

  it('clears history', () => {
    const { result } = renderHook(() => useMetricsHistory())
    const metrics = createMockMetrics('2026-01-20T12:00:00Z')

    act(() => {
      result.current.addMetrics(metrics)
    })

    expect(result.current.history).toHaveLength(1)

    act(() => {
      result.current.clearHistory()
    })

    expect(result.current.history).toHaveLength(0)
  })

  it('maintains insertion order', () => {
    const { result } = renderHook(() => useMetricsHistory())

    act(() => {
      result.current.addMetrics(createMockMetrics('2026-01-20T12:00:00Z'))
      result.current.addMetrics(createMockMetrics('2026-01-20T12:01:00Z'))
      result.current.addMetrics(createMockMetrics('2026-01-20T12:02:00Z'))
    })

    expect(result.current.history).toHaveLength(3)
    expect(result.current.history[0].timestamp).toBe('2026-01-20T12:00:00Z')
    expect(result.current.history[1].timestamp).toBe('2026-01-20T12:01:00Z')
    expect(result.current.history[2].timestamp).toBe('2026-01-20T12:02:00Z')
  })
})
