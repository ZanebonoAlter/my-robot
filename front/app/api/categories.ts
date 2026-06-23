import { apiClient } from './client'
import { camelizeKeys, mapApiResponse } from '~/utils/api-helpers'
import type { ApiResponse, CreateCategoryData, UpdateCategoryData, Category } from '~/types'

interface CategoryPayload {
  id: number
  name: string
  slug: string
  icon: string
  color: string
  description: string
  feed_count: number
}

function normalizeCategory(cat: CategoryPayload): Category {
  const c = camelizeKeys<Category>(cat)
  return {
    ...c,
    id: String(cat.id),
    slug: c.slug || cat.name.toLowerCase().replace(/\s+/g, '-'),
    icon: c.icon || 'mdi:folder',
    color: c.color || '#6b7280',
  }
}

export function useCategoriesApi() {
  async function getCategories(): Promise<ApiResponse<Category[]>> {
    const response = await apiClient.get<CategoryPayload[]>('/categories')
    if (response.success && response.data) {
      return {
        ...response,
        data: response.data.map(normalizeCategory),
      }
    }
    return { ...response, data: [] as Category[] }
  }

  async function createCategory(data: CreateCategoryData): Promise<ApiResponse<Category>> {
    const response = await apiClient.post<CategoryPayload>('/categories', data)
    if (response.success && response.data) {
      return mapApiResponse(response, normalizeCategory(response.data))
    }
    return { success: false, error: response.error }
  }

  async function updateCategory(id: number, data: UpdateCategoryData): Promise<ApiResponse<Category>> {
    const response = await apiClient.put<CategoryPayload>(`/categories/${id}`, data)
    if (response.success && response.data) {
      return mapApiResponse(response, normalizeCategory(response.data))
    }
    return { success: false, error: response.error }
  }

  async function deleteCategory(id: number): Promise<ApiResponse<void>> {
    return apiClient.delete<void>(`/categories/${id}`)
  }

  return {
    getCategories,
    createCategory,
    updateCategory,
    deleteCategory,
  }
}
