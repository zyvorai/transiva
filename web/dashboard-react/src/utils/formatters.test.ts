// SPDX-License-Identifier: Apache-2.0

import { describe, it, expect } from 'vitest'
import {
  formatBytes,
  formatDuration,
  formatPercentage,
  formatNumber,
  formatTimestamp,
  formatRelativeTime,
  getStatusColor,
  getStatusIcon,
  getSeverityColor,
} from './formatters'

describe('formatBytes', () => {
  it('formats 0 bytes', () => {
    expect(formatBytes(0)).toBe('0 B')
  })

  it('formats bytes correctly', () => {
    expect(formatBytes(512)).toBe('512 B')
  })

  it('formats kilobytes correctly', () => {
    expect(formatBytes(1024)).toBe('1 KB')
    expect(formatBytes(2048)).toBe('2 KB')
  })

  it('formats megabytes correctly', () => {
    expect(formatBytes(1048576)).toBe('1 MB')
    expect(formatBytes(5242880)).toBe('5 MB')
  })

  it('formats gigabytes correctly', () => {
    expect(formatBytes(1073741824)).toBe('1 GB')
    expect(formatBytes(2147483648)).toBe('2 GB')
  })

  it('formats terabytes correctly', () => {
    expect(formatBytes(1099511627776)).toBe('1 TB')
  })
})

describe('formatDuration', () => {
  it('formats seconds', () => {
    expect(formatDuration(30)).toBe('30s')
    expect(formatDuration(59)).toBe('59s')
  })

  it('formats minutes and seconds', () => {
    expect(formatDuration(60)).toBe('1m 0s')
    expect(formatDuration(90)).toBe('1m 30s')
    expect(formatDuration(3599)).toBe('59m 59s')
  })

  it('formats hours and minutes', () => {
    expect(formatDuration(3600)).toBe('1h 0m')
    expect(formatDuration(7200)).toBe('2h 0m')
    expect(formatDuration(5400)).toBe('1h 30m')
  })

  it('formats days and hours', () => {
    expect(formatDuration(86400)).toBe('1d 0h')
    expect(formatDuration(172800)).toBe('2d 0h')
    expect(formatDuration(90000)).toBe('1d 1h')
  })
})

describe('formatPercentage', () => {
  it('formats percentage with default decimals', () => {
    expect(formatPercentage(50)).toBe('50.0%')
    expect(formatPercentage(75.5)).toBe('75.5%')
  })

  it('formats percentage with custom decimals', () => {
    expect(formatPercentage(50, 0)).toBe('50%')
    expect(formatPercentage(75.567, 2)).toBe('75.57%')
  })
})

describe('formatNumber', () => {
  it('formats numbers with default decimals', () => {
    expect(formatNumber(1000)).toBe('1,000')
    expect(formatNumber(1000000)).toBe('1,000,000')
  })

  it('formats numbers with decimals', () => {
    expect(formatNumber(1234.567, 2)).toBe('1,234.57')
  })
})

describe('getStatusColor', () => {
  it('returns correct colors for job statuses', () => {
    expect(getStatusColor('pending')).toBe('#6b7280')
    expect(getStatusColor('running')).toBe('#3b82f6')
    expect(getStatusColor('completed')).toBe('#10b981')
    expect(getStatusColor('failed')).toBe('#ef4444')
    expect(getStatusColor('cancelled')).toBe('#f59e0b')
  })

  it('returns correct colors for health statuses', () => {
    expect(getStatusColor('healthy')).toBe('#10b981')
    expect(getStatusColor('degraded')).toBe('#f59e0b')
    expect(getStatusColor('unhealthy')).toBe('#ef4444')
  })

  it('returns default color for unknown status', () => {
    expect(getStatusColor('unknown')).toBe('#6b7280')
  })
})

describe('getStatusIcon', () => {
  it('returns correct icons for statuses', () => {
    expect(getStatusIcon('pending')).toBe('⏳')
    expect(getStatusIcon('running')).toBe('▶️')
    expect(getStatusIcon('completed')).toBe('✅')
    expect(getStatusIcon('failed')).toBe('❌')
    expect(getStatusIcon('cancelled')).toBe('🚫')
  })

  it('returns default icon for unknown status', () => {
    expect(getStatusIcon('unknown')).toBe('●')
  })
})

describe('getSeverityColor', () => {
  it('returns correct colors for severity levels', () => {
    expect(getSeverityColor('info')).toBe('#3b82f6')
    expect(getSeverityColor('warning')).toBe('#f59e0b')
    expect(getSeverityColor('error')).toBe('#ef4444')
    expect(getSeverityColor('critical')).toBe('#991b1b')
  })

  it('returns default color for unknown severity', () => {
    expect(getSeverityColor('unknown')).toBe('#6b7280')
  })
})

describe('formatTimestamp', () => {
  it('formats ISO timestamp to locale string', () => {
    const timestamp = '2026-01-21T10:00:00Z'
    const result = formatTimestamp(timestamp)

    // Result will vary by locale, just check it's a non-empty string
    expect(result).toBeDefined()
    expect(typeof result).toBe('string')
    expect(result.length).toBeGreaterThan(0)
  })
})

describe('formatRelativeTime', () => {
  it('formats time less than 60 seconds ago', () => {
    const now = new Date()
    const timestamp = new Date(now.getTime() - 30000).toISOString() // 30 seconds ago
    const result = formatRelativeTime(timestamp)

    expect(result).toMatch(/\d+s ago/)
  })

  it('formats time in minutes', () => {
    const now = new Date()
    const timestamp = new Date(now.getTime() - 120000).toISOString() // 2 minutes ago
    const result = formatRelativeTime(timestamp)

    expect(result).toBe('2m ago')
  })

  it('formats time in hours', () => {
    const now = new Date()
    const timestamp = new Date(now.getTime() - 7200000).toISOString() // 2 hours ago
    const result = formatRelativeTime(timestamp)

    expect(result).toBe('2h ago')
  })

  it('formats time in days', () => {
    const now = new Date()
    const timestamp = new Date(now.getTime() - 172800000).toISOString() // 2 days ago
    const result = formatRelativeTime(timestamp)

    expect(result).toBe('2d ago')
  })
})
