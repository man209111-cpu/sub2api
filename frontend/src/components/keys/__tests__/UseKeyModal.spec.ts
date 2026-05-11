import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key
  })
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({
    copyToClipboard: vi.fn().mockResolvedValue(true)
  })
}))

import UseKeyModal from '../UseKeyModal.vue'

describe('UseKeyModal', () => {
  it('renders GPT-5.4 mini entry in OpenCode config', async () => {
    const wrapper = mount(UseKeyModal, {
      props: {
        show: true,
        apiKey: 'sk-test',
        baseUrl: 'https://example.com/v1',
        platform: 'openai'
      },
      global: {
        stubs: {
          BaseDialog: {
            template: '<div><slot /><slot name="footer" /></div>'
          },
          Icon: {
            template: '<span />'
          }
        }
      }
    })

    const opencodeTab = wrapper.findAll('button').find((button) =>
      button.text().includes('keys.useKeyModal.cliTabs.opencode')
    )

    expect(opencodeTab).toBeDefined()
    await opencodeTab!.trigger('click')
    await nextTick()

    const codeBlock = wrapper.find('pre code')
    expect(codeBlock.exists()).toBe(true)
    expect(codeBlock.text()).toContain('"name": "GPT-5.4 Mini"')
    expect(codeBlock.text()).not.toContain('"name": "GPT-5.4 Nano"')
  })

  it.each(['anthropic', 'openai', 'gemini', 'antigravity'] as const)(
    'renders OpenClaw config for %s keys',
    async (platform) => {
      const wrapper = mount(UseKeyModal, {
        props: {
          show: true,
          apiKey: 'sk-test',
          baseUrl: 'https://example.com/v1',
          platform
        },
        global: {
          stubs: {
            BaseDialog: {
              template: '<div><slot /><slot name="footer" /></div>'
            },
            Icon: {
              template: '<span />'
            }
          }
        }
      })

      const openclawTab = wrapper.findAll('button').find((button) =>
        button.text().includes('keys.useKeyModal.cliTabs.openclaw')
      )

      expect(openclawTab).toBeDefined()
      await openclawTab!.trigger('click')
      await nextTick()

      expect(wrapper.text()).toContain('~/.openclaw/openclaw.json')

      const codeBlock = wrapper.find('pre code')
      expect(codeBlock.exists()).toBe(true)
      expect(codeBlock.text()).toContain('"models":')

      if (platform === 'openai') {
        expect(codeBlock.text()).toContain('"api": "openai-responses"')
      } else if (platform === 'gemini') {
        expect(codeBlock.text()).toContain('"api": "google-generative-ai"')
      } else if (platform === 'antigravity') {
        expect(codeBlock.text()).toContain('sub2api-antigravity-claude')
        expect(codeBlock.text()).toContain('sub2api-antigravity-gemini')
      } else {
        expect(codeBlock.text()).toContain('"api": "anthropic-messages"')
      }
    }
  )
})
