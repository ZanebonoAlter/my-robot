## 1. Install driver.js

- [ ] 1.1 Run `pnpm add driver.js` in `front/` to add the guided-tour library

## 2. Create useOnboarding composable

- [ ] 2.1 Create `front/app/composables/useOnboarding.ts` with composable interface: `isFirstRun`, `isTourActive`, `startTour()`, `dismissTour()`, `isFeatureTipShown(tipId)`, `markFeatureTipShown(tipId)`, `resetOnboarding()`
- [ ] 2.2 Implement first-run detection: check `localStorage.getItem('syntopica_onboarding_complete')` to determine `isFirstRun`
- [ ] 2.3 Implement tour initialization: create driver.js instance inside `onMounted` (avoid SSR `document` access), define 5 steps using `data-onboarding` CSS selectors (welcome overlay, sidebar feeds, topic graph nav, tag management nav, watched tags/daily report)
- [ ] 2.4 Implement `dismissTour()`: set `syntopica_onboarding_complete` to `"true"`, destroy driver instance
- [ ] 2.5 Implement feature-tip tracking: read/write `syntopica_feature_tips` JSON in localStorage for `isFeatureTipShown()` / `markFeatureTipShown()`
- [ ] 2.6 Implement `resetOnboarding()`: clear both localStorage keys, call `location.reload()` to re-trigger tour

## 3. Add data-onboarding attributes to tour targets

- [ ] 3.1 Add `data-onboarding="sidebar-feeds"` to the feeds/categories section in `front/app/components/app/sidebar/AppSidebarView.vue`
- [ ] 3.2 Add `data-onboarding="topic-graph-nav"` to the topic graph navigation entry in `front/app/components/app/sidebar/AppSidebarView.vue`
- [ ] 3.3 Add `data-onboarding="tag-management-nav"` to the tag management navigation entry in `front/app/components/app/sidebar/AppSidebarView.vue`
- [ ] 3.4 Add `data-onboarding="watched-tags"` to the watched tags section in the sidebar

## 4. Empty state components

- [ ] 4.1 Create `front/app/features/feeds/components/FeedEmptyGuide.vue` — shown when no feeds and no categories exist, with "添加你的第一个 RSS 源" message and action button
- [ ] 4.2 Create `front/app/features/topic-graph/components/TopicGraphEmptyGuide.vue` — shown when graph has no data, with "等待标签数据积累" explanation
- [ ] 4.3 Create daily report empty state component — shown when no daily report exists, with "日报需要积累数据" explanation; place in the appropriate features directory

## 5. Feature discovery tooltips

- [ ] 5.1 Add `data-feature-tip="topic-graph-filter"` attribute to the topic graph filter controls in `front/app/features/topic-graph/`
- [ ] 5.2 Add `data-feature-tip="tag-merge-suggestions"` attribute to the tag merge suggestions UI in `front/app/features/tags/`
- [ ] 5.3 Add `data-feature-tip="daily-report-timeline"` attribute to the daily report timeline control
- [ ] 5.4 Implement lightweight CSS tooltip system: on page mount, check `syntopica_feature_tips` for unshown tips, display Tailwind + CSS transition tooltips on `data-feature-tip` elements, mark as shown on first display

## 6. Settings replay button

- [ ] 6.1 Add a "重播引导" button to the settings/preferences page that calls `useOnboarding().resetOnboarding()` to re-trigger the onboarding tour

## 7. prefers-reduced-motion support

- [ ] 7.1 In `useOnboarding.ts`, check `window.matchMedia('(prefers-reduced-motion: reduce)')` and pass `animate: false` to driver.js config when the user prefers reduced motion
- [ ] 7.2 Ensure feature discovery tooltips (CSS transitions) include `@media (prefers-reduced-motion: reduce)` to disable animations

## 8. Verify

- [ ] 8.1 Run `cd front && pnpm lint` — no new warnings or errors
- [ ] 8.2 Run `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm exec nuxi typecheck"` — passes
- [ ] 8.3 Manual test: clear `syntopica_onboarding_complete` in localStorage, reload page, verify 5-step tour launches and completes correctly
- [ ] 8.4 Manual test: verify "重播引导" button in settings re-triggers the tour
- [ ] 8.5 Manual test: verify empty state components display when respective data is absent
- [ ] 8.6 Manual test: verify feature discovery tooltips appear once and are suppressed on subsequent visits
