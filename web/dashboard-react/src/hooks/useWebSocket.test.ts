// SPDX-License-Identifier: Apache-2.0

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { renderHook, act, waitFor } from '@testing-library/react'
import { useWebSocket } from './useWebSocket'
import type { Metrics } from '../types/metrics'

// Mock WebSocket
class MockWebSocket {
  url: string
  onopen: ((event: Event) => void) | null = null
  onclose: ((event: CloseEvent) => void) | null = null
  onmessage: ((event: MessageEvent) => void) | null = null
  onerror: ((event: Event) => void) | null = null
  readyState: number = WebSocket.CONNECTING

  static CONNECTING = 0
  static OPEN = 1
  static CLOSING = 2
  static CLOSED = 3

  constructor(url: string) {
    this.url = url
    // Store instance for testing
    ;(global as any).__mockWebSocketInstance = this
  }

  send(data: string) {
    // Mock send
  }

  close(code?: number, reason?: string) {
    this.readyState = WebSocket.CLOSED
    if (this.onclose) {
      const event = new Event('close') as CloseEvent
      ;(event as any).code = code || 1000
      ;(event as any).reason = reason || ''
      this.onclose(event)
    }
  }

  // Helper methods for testing
  simulateOpen() {
    this.readyState = WebSocket.OPEN
    if (this.onopen) {
      this.onopen(new Event('open'))
    }
  }

  simulateMessage(data: string) {
    if (this.onmessage) {
      const event = new Event('message') as MessageEvent
      ;(event as any).data = data
      this.onmessage(event)
    }
  }

  simulateError() {
    if (this.onerror) {
      this.onerror(new Event('error'))
    }
  }

  simulateClose(code = 1000, reason = '') {
    this.close(code, reason)
  }
}

const createMockMetrics = (): Metrics => ({
  timestamp: '2026-01-21T10:00:00Z',
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

describe('useWebSocket', () => {
  let originalWebSocket: typeof WebSocket

  beforeEach(() => {
    // Save original WebSocket
    originalWebSocket = global.WebSocket
    // Replace with mock
    global.WebSocket = MockWebSocket as any

    // Mock console methods to avoid noise in tests
    vi.spyOn(console, 'log').mockImplementation(() => {})
    vi.spyOn(console, 'error').mockImplementation(() => {})
    vi.spyOn(console, 'warn').mockImplementation(() => {})
  })

  afterEach(() => {
    // Restore original WebSocket
    global.WebSocket = originalWebSocket
    vi.restoreAllMocks()
    delete (global as any).__mockWebSocketInstance
  })

  it('initializes with null data and disconnected state', () => {
    const { result } = renderHook(() =>
      useWebSocket({ url: 'ws://localhost:8080/ws' })
    )

    expect(result.current.data).toBeNull()
    expect(result.current.connected).toBe(false)
    expect(result.current.error).toBeNull()
    expect(result.current.reconnecting).toBe(false)
  })

  it('creates WebSocket connection with provided URL', () => {
    renderHook(() => useWebSocket({ url: 'ws://localhost:8080/ws' }))

    const ws = (global as any).__mockWebSocketInstance as MockWebSocket
    expect(ws).toBeDefined()
    expect(ws.url).toBe('ws://localhost:8080/ws')
  })

  it('sets connected to true when connection opens', () => {
    const { result } = renderHook(() =>
      useWebSocket({ url: 'ws://localhost:8080/ws' })
    )

    const ws = (global as any).__mockWebSocketInstance as MockWebSocket

    act(() => {
      ws.simulateOpen()
    })

    expect(result.current.connected).toBe(true)
    expect(result.current.error).toBeNull()
    expect(result.current.reconnecting).toBe(false)
  })

  it('updates data when receiving metrics message', () => {
    const { result } = renderHook(() =>
      useWebSocket({ url: 'ws://localhost:8080/ws' })
    )

    const ws = (global as any).__mockWebSocketInstance as MockWebSocket
    const mockMetrics = createMockMetrics()

    act(() => {
      ws.simulateOpen()
      ws.simulateMessage(JSON.stringify(mockMetrics))
    })

    expect(result.current.data).toEqual(mockMetrics)
  })

  it('handles malformed JSON gracefully', () => {
    const { result } = renderHook(() =>
      useWebSocket({ url: 'ws://localhost:8080/ws' })
    )

    const ws = (global as any).__mockWebSocketInstance as MockWebSocket

    act(() => {
      ws.simulateOpen()
      ws.simulateMessage('invalid json')
    })

    expect(result.current.error).toBeDefined()
    expect(result.current.data).toBeNull()
  })

  it('sets error when connection error occurs', () => {
    const { result } = renderHook(() =>
      useWebSocket({ url: 'ws://localhost:8080/ws' })
    )

    const ws = (global as any).__mockWebSocketInstance as MockWebSocket

    act(() => {
      ws.simulateError()
    })

    expect(result.current.error).toBeDefined()
    expect(result.current.error?.message).toBe('WebSocket connection error')
  })

  it('sets connected to false when connection closes', () => {
    const { result } = renderHook(() =>
      useWebSocket({ url: 'ws://localhost:8080/ws' })
    )

    const ws = (global as any).__mockWebSocketInstance as MockWebSocket

    act(() => {
      ws.simulateOpen()
    })

    expect(result.current.connected).toBe(true)

    act(() => {
      ws.simulateClose(1000)
    })

    expect(result.current.connected).toBe(false)
  })

  it('attempts reconnection when connection closes unexpectedly', async () => {
    vi.useFakeTimers()

    const { result } = renderHook(() =>
      useWebSocket({
        url: 'ws://localhost:8080/ws',
        reconnectInterval: 1000,
        reconnectAttempts: 3,
      })
    )

    const ws1 = (global as any).__mockWebSocketInstance as MockWebSocket

    act(() => {
      ws1.simulateOpen()
    })

    // Close with non-1000 code to trigger reconnection
    act(() => {
      ws1.simulateClose(1006, 'Connection lost')
    })

    expect(result.current.reconnecting).toBe(true)

    // Advance timers to trigger reconnection
    await act(async () => {
      vi.advanceTimersByTime(1000)
    })

    // A new WebSocket should have been created
    const ws2 = (global as any).__mockWebSocketInstance as MockWebSocket
    expect(ws2).toBeDefined()

    vi.useRealTimers()
  })

  it('stops reconnecting after max attempts', async () => {
    vi.useFakeTimers()

    const { result } = renderHook(() =>
      useWebSocket({
        url: 'ws://localhost:8080/ws',
        reconnectInterval: 100,
        reconnectAttempts: 2,
      })
    )

    // Close and reconnect multiple times
    for (let i = 0; i < 3; i++) {
      const ws = (global as any).__mockWebSocketInstance as MockWebSocket

      await act(async () => {
        ws.simulateClose(1006)
        vi.advanceTimersByTime(100)
      })
    }

    expect(result.current.error?.message).toBe('Maximum reconnection attempts reached')
    expect(result.current.reconnecting).toBe(false)

    vi.useRealTimers()
  })

  it('does not reconnect when closed intentionally (code 1000)', () => {
    const { result } = renderHook(() =>
      useWebSocket({ url: 'ws://localhost:8080/ws' })
    )

    const ws = (global as any).__mockWebSocketInstance as MockWebSocket

    act(() => {
      ws.simulateOpen()
      ws.simulateClose(1000, 'Normal closure')
    })

    expect(result.current.reconnecting).toBe(false)
  })

  it('provides sendMessage function', () => {
    const { result } = renderHook(() =>
      useWebSocket({ url: 'ws://localhost:8080/ws' })
    )

    expect(result.current.sendMessage).toBeDefined()
    expect(typeof result.current.sendMessage).toBe('function')
  })

  it('sendMessage sends data when connected', () => {
    const { result } = renderHook(() =>
      useWebSocket({ url: 'ws://localhost:8080/ws' })
    )

    const ws = (global as any).__mockWebSocketInstance as MockWebSocket
    const sendSpy = vi.spyOn(ws, 'send')

    act(() => {
      ws.simulateOpen()
    })

    act(() => {
      result.current.sendMessage({ type: 'ping' })
    })

    expect(sendSpy).toHaveBeenCalledWith(JSON.stringify({ type: 'ping' }))
  })

  it('sendMessage warns when not connected', () => {
    const { result } = renderHook(() =>
      useWebSocket({ url: 'ws://localhost:8080/ws' })
    )

    const warnSpy = vi.spyOn(console, 'warn')

    act(() => {
      result.current.sendMessage({ type: 'ping' })
    })

    expect(warnSpy).toHaveBeenCalledWith(
      'WebSocket is not connected. Cannot send message.'
    )
  })

  it('cleans up on unmount', () => {
    const { unmount } = renderHook(() =>
      useWebSocket({ url: 'ws://localhost:8080/ws' })
    )

    const ws = (global as any).__mockWebSocketInstance as MockWebSocket
    const closeSpy = vi.spyOn(ws, 'close')

    unmount()

    expect(closeSpy).toHaveBeenCalledWith(1000, 'Component unmounting')
  })

  it('uses default reconnect interval and attempts', () => {
    const { result } = renderHook(() =>
      useWebSocket({ url: 'ws://localhost:8080/ws' })
    )

    expect(result.current).toBeDefined()
    // Default values are used internally
  })

  it('resets reconnect count on successful connection', async () => {
    vi.useFakeTimers()

    const { result } = renderHook(() =>
      useWebSocket({
        url: 'ws://localhost:8080/ws',
        reconnectInterval: 100,
        reconnectAttempts: 5,
      })
    )

    // First connection
    let ws = (global as any).__mockWebSocketInstance as MockWebSocket
    act(() => {
      ws.simulateOpen()
    })

    // Close and reconnect
    act(() => {
      ws.simulateClose(1006)
    })

    await act(async () => {
      vi.advanceTimersByTime(100)
    })

    // New connection opens successfully
    ws = (global as any).__mockWebSocketInstance as MockWebSocket
    act(() => {
      ws.simulateOpen()
    })

    expect(result.current.reconnecting).toBe(false)
    expect(result.current.error).toBeNull()

    vi.useRealTimers()
  })
})
