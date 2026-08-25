import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import ArticleTagList from './ArticleTagList.vue'

describe('ArticleTagList', () => {
  it('renders aggregated article tags and highlights matched slugs', () => {
    const wrapper = mount(ArticleTagList, {
      props: {
        tags: [
          { slug: 'ai-agent', label: 'AI Agent', category: 'keyword', articleCount: 2 },
          { slug: 'sam-altman', label: 'Sam Altman', category: 'person', articleCount: 1 },
        ],
        highlightedSlugs: ['ai-agent'],
      },
    })

    expect(wrapper.text()).toContain('AI Agent')
    expect(wrapper.text()).toContain('Sam Altman')
    expect(wrapper.find('[data-tag-slug="ai-agent"]').classes()).toContain('article-tag--highlighted')
  })

  it('expands and collapses hidden tags when clicking the +N badge in compact mode', async () => {
    const wrapper = mount(ArticleTagList, {
      props: {
        compact: true,
        maxVisible: 2,
        showWatch: true,
        tags: [
          { id: 1, slug: 'ai-agent', label: 'AI Agent', category: 'keyword', articleCount: 2 },
          { id: 2, slug: 'openai', label: 'OpenAI', category: 'keyword', articleCount: 2 },
          { id: 3, slug: 'gpt-5', label: 'GPT-5', category: 'keyword', articleCount: 1 },
        ],
      },
    })

    const badge = wrapper.find('.article-tag--more')
    expect(badge.exists()).toBe(true)
    expect(badge.text()).toBe('+1')
    expect(wrapper.find('[data-tag-slug="gpt-5"]').exists()).toBe(false)

    await badge.trigger('click')
    expect(wrapper.find('[data-tag-slug="gpt-5"]').exists()).toBe(true)
    expect(wrapper.find('[data-tag-slug="gpt-5"] .article-tag__watch').exists()).toBe(true)

    await badge.trigger('click')
    expect(wrapper.find('[data-tag-slug="gpt-5"]').exists()).toBe(false)
    expect(wrapper.find('.article-tag--more').text()).toBe('+1')
  })

  it('truncates long tag lists in compact mode', () => {
    const wrapper = mount(ArticleTagList, {
      props: {
        compact: true,
        maxVisible: 2,
        tags: [
          { slug: 'ai-agent', label: 'AI Agent', category: 'keyword', articleCount: 2 },
          { slug: 'openai', label: 'OpenAI', category: 'keyword', articleCount: 2 },
          { slug: 'gpt-5', label: 'GPT-5', category: 'keyword', articleCount: 1 },
        ],
      },
    })

    expect(wrapper.text()).toContain('AI Agent')
    expect(wrapper.text()).toContain('OpenAI')
    expect(wrapper.text()).toContain('+1')
  })
})
