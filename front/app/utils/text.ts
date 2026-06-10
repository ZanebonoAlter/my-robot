/**
 * 文本工具函数
 */

/**
 * 截断文本
 * @param text - 原始文本
 * @param maxLength - 最大长度
 * @returns 截断后的文本
 */
export function truncateText(text: string, maxLength: number): string {
  if (text.length <= maxLength) return text
  return text.substring(0, maxLength) + '...'
}

/**
 * 高亮关键词
 * @param text - 原始文本
 * @param keyword - 关键词
 * @returns 高亮后的 HTML 字符串
 */
export function highlightKeyword(text: string, keyword: string): string {
  if (!keyword) return text
  const regex = new RegExp(`(${keyword})`, 'gi')
  return text.replace(regex, '<mark>$1</mark>')
}

