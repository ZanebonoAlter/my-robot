/**
 * 日期工具函数
 */

/**
 * 格式化日期
 * @param dateString - 日期字符串
 * @param format - 格式字符串，默认为 'YYYY年MM月DD日 HH:mm'
 * @returns 格式化后的日期字符串
 */
export function formatDate(
  dateString: string,
  format: string = 'YYYY年MM月DD日 HH:mm'
): string {
  const date = new Date(dateString)
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  const hours = String(date.getHours()).padStart(2, '0')
  const minutes = String(date.getMinutes()).padStart(2, '0')

  return format
    .replace('YYYY', String(year))
    .replace('MM', month)
    .replace('DD', day)
    .replace('HH', hours)
    .replace('mm', minutes)
}
