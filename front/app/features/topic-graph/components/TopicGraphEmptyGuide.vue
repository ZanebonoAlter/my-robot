<script setup lang="ts">
import { Icon } from '@iconify/vue'

/**
 * TopicGraphEmptyGuide — 主题图谱空状态引导。
 *
 * 图谱无数据（无标签或无标签关系）时由父组件条件渲染（`v-if`），
 * 解释空的原因（需要先积累标签数据）并给出建议。
 *
 * 详见 openspec/changes/user-onboarding/spec.md「Empty state guide for topic graph」。
 */
</script>

<template>
  <div class="topic-graph-empty-guide">
    <Icon icon="mdi:graph-outline" width="40" height="40" class="topic-graph-empty-guide__icon" />
    <p class="topic-graph-empty-guide__title">等待标签数据积累</p>
    <p class="topic-graph-empty-guide__desc">
      主题图谱会随文章的抓取与 AI 打标逐步生成。先添加 RSS 源并等待内容抓取，
      标签及其关系积累后，图谱会自动呈现。
    </p>
  </div>
</template>

<style scoped>
.topic-graph-empty-guide {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.5rem;
  padding: 3rem 1.5rem;
  text-align: center;
}
.topic-graph-empty-guide__icon {
  color: var(--color-text-muted);
  opacity: 0.5;
}
.topic-graph-empty-guide__title {
  font-size: 0.95rem;
  font-weight: 600;
  color: var(--color-text-primary);
  margin: 0;
}
.topic-graph-empty-guide__desc {
  font-size: 0.8rem;
  color: var(--color-text-muted);
  margin: 0;
  line-height: 1.6;
  max-width: 28rem;
}
</style>
