import { ref, onBeforeUnmount } from 'vue'

export function useFloatingPanelDrag() {
  const panelRef = ref<HTMLDivElement | null>(null)
  const isDragging = ref(false)
  const dragOffset = ref({ x: 0, y: 0 })
  const position = ref({ x: 0, y: 0 })
  const initialRect = ref({ left: 0, top: 0 })
  let initialPositionSet = false

  function startDrag(event: MouseEvent) {
    const target = event.target as HTMLElement
    if (target.closest('button')) return
    const rect = panelRef.value?.getBoundingClientRect()
    if (!rect) return
    if (!initialPositionSet) {
      initialRect.value = { left: rect.left, top: rect.top }
      initialPositionSet = true
    }
    isDragging.value = true
    dragOffset.value = {
      x: event.clientX - rect.left,
      y: event.clientY - rect.top,
    }
    event.preventDefault()
    document.addEventListener('mousemove', handleDrag)
    document.addEventListener('mouseup', endDrag)
  }

  function handleDrag(event: MouseEvent) {
    if (!isDragging.value) return
    position.value = {
      x: event.clientX - dragOffset.value.x - initialRect.value.left,
      y: event.clientY - dragOffset.value.y - initialRect.value.top,
    }
  }

  function endDrag() {
    isDragging.value = false
    document.removeEventListener('mousemove', handleDrag)
    document.removeEventListener('mouseup', endDrag)
  }

  function resetPosition() {
    position.value = { x: 0, y: 0 }
    initialPositionSet = false
    initialRect.value = { left: 0, top: 0 }
  }

  onBeforeUnmount(() => {
    document.removeEventListener('mousemove', handleDrag)
    document.removeEventListener('mouseup', endDrag)
  })

  return { panelRef, isDragging, position, startDrag, resetPosition }
}
