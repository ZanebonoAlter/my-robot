import { ref } from 'vue'
import { useArticlesApi } from '~/api/articles'
import type { Article } from '~/types'
import { normalizeArticle, type ArticlePayload } from '~/api/normalizers/article'

export function useArticlePreview() {
  const articlesApi = useArticlesApi()

  const selectedPreviewArticle = ref<Article | null>(null)
  const previewArticles = ref<Article[]>([])
  const loadingPreviewArticle = ref(false)

  async function openArticlePreview(articleId: number, relatedArticleIds?: number[]) {
    loadingPreviewArticle.value = true
    try {
      const response = await articlesApi.getArticle(articleId)
      if (!response.success || !response.data) return
      selectedPreviewArticle.value = normalizeArticle(response.data as unknown as ArticlePayload)

      if (relatedArticleIds?.length) {
        const uniqueIds = Array.from(new Set(relatedArticleIds))
        const articleResponses = await Promise.all(uniqueIds.slice(0, 12).map(id => articlesApi.getArticle(id)))
        previewArticles.value = articleResponses
          .filter(item => item.success && item.data)
          .map(item => normalizeArticle(item.data as unknown as ArticlePayload))
      }
    } catch (error) {
      console.error('Failed to open article preview:', error)
    } finally {
      loadingPreviewArticle.value = false
    }
  }

  function closeArticlePreview() {
    selectedPreviewArticle.value = null
  }

  async function handleArticleFavorite(articleId: string) {
    const currentFavorite = selectedPreviewArticle.value?.id === articleId
      ? selectedPreviewArticle.value.favorite
      : previewArticles.value.find(a => a.id === articleId)?.favorite
    const response = await articlesApi.updateArticle(Number(articleId), { favorite: !currentFavorite })
    if (!response.success) return
    const target = previewArticles.value.find(article => article.id === articleId)
    if (target) target.favorite = !target.favorite
    if (selectedPreviewArticle.value?.id === articleId) {
      selectedPreviewArticle.value = { ...selectedPreviewArticle.value, favorite: !selectedPreviewArticle.value.favorite }
    }
  }

  function handleArticleUpdate(articleId: string, updates: Partial<Article>) {
    const target = previewArticles.value.find(article => article.id === articleId)
    if (target) Object.assign(target, updates)
    if (selectedPreviewArticle.value?.id === articleId) {
      Object.assign(selectedPreviewArticle.value, updates)
    }
  }

  return {
    selectedPreviewArticle, previewArticles, loadingPreviewArticle,
    openArticlePreview, closeArticlePreview,
    handleArticleFavorite, handleArticleUpdate,
  }
}
