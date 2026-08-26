// SPDX-License-Identifier: Apache-2.0

// Test setup file for Vitest
import { afterEach } from 'vitest'
import { cleanup } from '@testing-library/react'

// Cleanup after each test
afterEach(() => {
  cleanup()
})
