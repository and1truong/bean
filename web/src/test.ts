import '@testing-library/jest-dom'

class TestResizeObserver {
  observe() {}
  unobserve() {}
  disconnect() {}
}

globalThis.ResizeObserver=TestResizeObserver
