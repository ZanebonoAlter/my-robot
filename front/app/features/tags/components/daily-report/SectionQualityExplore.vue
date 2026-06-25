<script setup lang="ts">
import { matchReasonColor, matchInfoLabel } from '~/utils/matchQuality'
import type { DailyReportQualityEntry } from '~/api/dailyReports'

/**
 * Quality probe panel for a daily-report section. Renders the frozen
 * `quality_breakdown` lineage — one chip per source tag, coloured by match
 * reason (theme token) with the score and a ↓ for downgraded tags.
 *
 * Historical sections (quality_breakdown = null) show a "无质量明细" placeholder.
 * Pure display: the parent reveals it on hover/focus (see DailyReportTopicSection).
 */
defineProps<{ breakdown?: DailyReportQualityEntry[] | null }>()
</script>

<template>
  <div class="section-quality-explore">
    <ul v-if="breakdown && breakdown.length" class="section-quality-explore__list">
      <li
        v-for="entry in breakdown"
        :key="entry.tag_id"
        class="section-quality-explore__chip"
        :class="{ 'section-quality-explore__chip--downgraded': entry.downgraded }"
        :data-tag-id="entry.tag_id"
        :style="{ borderColor: matchReasonColor(entry.match_reason, entry.downgraded) }"
      >
        <span class="section-quality-explore__name">{{ entry.label }}</span>
        <span class="section-quality-explore__meta">{{ matchInfoLabel(entry) }}</span>
      </li>
    </ul>
    <p v-else class="section-quality-explore__empty">无质量明细</p>
  </div>
</template>

<style scoped>
.section-quality-explore {
  min-width: 14rem;
}

.section-quality-explore__list {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  margin: 0;
  padding: 0;
  list-style: none;
}

.section-quality-explore__chip {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 0.5rem;
  padding: 0.2rem 0.45rem;
  border: 1px solid;
  border-radius: 4px;
  background: var(--color-bg-hover);
  font-size: 0.68rem;
  line-height: 1.4;
}

.section-quality-explore__name {
  color: var(--color-text-secondary);
  font-weight: 500;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.section-quality-explore__meta {
  flex-shrink: 0;
  color: var(--color-text-muted);
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}

.section-quality-explore__empty {
  margin: 0;
  color: var(--color-text-muted);
  font-size: 0.68rem;
  font-style: italic;
}
</style>
