<script setup lang="ts">
import { computed, ref } from 'vue'
import { Icon } from '@iconify/vue'
import AppButton from '~/components/ui/AppButton.vue'
import AppDialog from '~/components/ui/AppDialog.vue'
import AppInput from '~/components/ui/AppInput.vue'
import { useTopicWatchesApi, type CreateWatchResult, type TopicWatchType } from '~/api/topicWatches'
import { isValidKeywordExpr, parseKeywordSlots } from './keywordExpr'

/**
 * 新建关注对话框（物化轨双选：关键字话题 keyword_topic / 一句话话题 sentence_topic）。
 *
 * watch-materialized-topic 收尾：旧提示轨（label / keyword）创建入口退役隐藏——
 * 存量关注继续在管理面板展示/管理，API 兼容保留，仅新建不再提供。
 * 设计决策锚 watch-keyword-and-quickadd design.md §4.5/§4.6（解析预览沿袭）：
 * - keyword_topic 复用 keyword 三件套：语法提示（空格=AND、|=OR）+ 实时解析预览
 *   （chips）+ 物化预期说明；无效表达式 → 预览红字 + 提交禁用（与后端 400 对齐）；
 * - sentence_topic：话题名必填 + 检索句可选（空则回退话题名），生效说明固定。
 *
 * 挂载方（WatchManagePanel）：v-model + @created（带 CreateWatchResult）。
 */
const props = defineProps<{
  modelValue: boolean
  boardId: number
}>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  /** 创建成功；keyword 类 result.instantHitCount = 即时回扫命中数。 */
  created: [result: CreateWatchResult]
}>()

const api = useTopicWatchesApi()

const visible = computed(() => props.modelValue)

// —— 类型双选（物化轨） ——
const watchType = ref<TopicWatchType>('keyword_topic')

// —— 输入态 ——
const keywordText = ref('')
const sentenceNameText = ref('') // sentence_topic 话题名（时间线展示）
const sentenceQueryText = ref('') // sentence_topic 检索句（embedding 输入，空则回退话题名）
const nameTouched = ref(false) // sentence 话题名空值误提交过一次才显错误提示（不打断初输）
const submitting = ref(false)
const submitError = ref<string | null>(null)

// —— keyword 解析预览 ——
const keywordSlots = computed(() => parseKeywordSlots(keywordText.value))
const keywordValid = computed(() => isValidKeywordExpr(keywordText.value))
/** keyword 态是否已开始输入（未输入时不显示红字，只显示提示区）。 */
const keywordTouched = computed(() => keywordText.value.trim().length > 0)
const keywordError = computed(() => {
  if (keywordTouched.value && !keywordValid.value) return '关键字表达式无效：空/纯分隔符无法解析出有效词组，提交将被拒绝'
  return null
})
const previewText = computed(() => keywordSlots.value.map(s => s.join('|')).join(' × '))

// —— 提交门禁 ——
const sentenceNameEmptyError = computed(() => {
  if (nameTouched.value && !sentenceNameText.value.trim()) return '请填写话题名（将作为日报中的话题标题）'
  return undefined
})
const canSubmit = computed(() => {
  if (submitting.value) return false
  if (watchType.value === 'sentence_topic') return !!sentenceNameText.value.trim()
  return keywordValid.value // keyword_topic
})

/** 双类型选项（物化轨；旧提示轨 label/keyword 创建入口退役隐藏）。 */
const typeOptions: Array<{ value: TopicWatchType, name: string, desc: string, icon: string }> = [
  { value: 'keyword_topic', name: '关键字话题', desc: '当天含词文章聚合板块', icon: 'mdi:text-box-search' },
  { value: 'sentence_topic', name: '一句话话题', desc: '语义检索 · 持久话题线', icon: 'mdi:cube-scan' },
]

function switchType(t: TopicWatchType) {
  if (watchType.value === t) return
  watchType.value = t
  // 切型清错误态：提交错误 / 空值 touched 标记不跨型残留（输入内容各型独立保留）
  submitError.value = null
  nameTouched.value = false
}

function close() {
  emit('update:modelValue', false)
}

/** 打开时重置态（面板复用同一实例多次新建）。 */
function resetOnOpen(v: boolean) {
  if (v) {
    watchType.value = 'keyword_topic'
    keywordText.value = ''
    sentenceNameText.value = ''
    sentenceQueryText.value = ''
    nameTouched.value = false
    submitError.value = null
  }
}
watch(visible, resetOnOpen)

async function submit() {
  if (!canSubmit.value) {
    // 空值误提交：显错误提示（输入内容保留，不重置）
    if (watchType.value === 'sentence_topic') nameTouched.value = true
    return
  }
  submitting.value = true
  submitError.value = null
  let res: Awaited<ReturnType<typeof api.createWatch>>
  if (watchType.value === 'sentence_topic') {
    res = await api.createWatch(
      props.boardId,
      sentenceNameText.value.trim(),
      'sentence_topic',
      sentenceQueryText.value.trim() || undefined, // 空检索句回退话题名（后端语义）
    )
  }
  else {
    // keyword_topic：表达式即 label
    res = await api.createWatch(props.boardId, keywordText.value.trim(), 'keyword_topic')
  }
  submitting.value = false
  if (res.success && res.data) {
    emit('created', res.data)
    emit('update:modelValue', false)
  }
  else {
    // 失败保留输入内容，仅提示（误输入反馈可见性约束）
    submitError.value = res.error ?? '创建失败，请稍后重试'
  }
}
</script>

<template>
  <AppDialog
    :model-value="visible"
    title="新建关注"
    width="460px"
    :close-on-overlay="false"
    @update:model-value="close"
  >
    <div class="twcd">
      <!-- 类型四选：提示轨（label/keyword）× 物化轨（keyword_topic/sentence_topic） -->
      <div class="twcd-type" role="radiogroup" aria-label="关注类型">
        <button
          v-for="opt in typeOptions"
          :key="opt.value"
          type="button"
          class="twcd-type__card"
          :class="{ 'is-on': watchType === opt.value }"
          role="radio"
          :aria-checked="watchType === opt.value"
          :data-testid="`watch-type-${opt.value}`"
          @click="switchType(opt.value)"
        >
          <Icon :icon="opt.icon" width="16" aria-hidden="true" />
          <span class="twcd-type__name">{{ opt.name }}</span>
          <span class="twcd-type__desc">{{ opt.desc }}</span>
        </button>
      </div>

      <!-- sentence_topic 态：话题名 + 检索句 -->
      <div v-if="watchType === 'sentence_topic'" class="twcd-field">
        <label class="twcd-field__label" for="twcd-st-name-input">话题名</label>
        <AppInput
          id="twcd-st-name-input"
          v-model="sentenceNameText"
          placeholder="如：AI 编程工具进展"
          :error="sentenceNameEmptyError"
          data-testid="watch-sentence-name-input"
          @keydown.enter="submit"
        />
        <label class="twcd-field__label twcd-field__label--second" for="twcd-st-query-input">检索句（可选）</label>
        <AppInput
          id="twcd-st-query-input"
          v-model="sentenceQueryText"
          placeholder="用于语义检索的句子，留空则用话题名，如：AI coding assistant 的进展和竞争格局"
          data-testid="watch-sentence-query-input"
          @keydown.enter="submit"
        />
        <p class="twcd-scan-note">
          <Icon icon="mdi:cube-scan" width="14" aria-hidden="true" />
          每期日报按检索句的语义检索相关辅助标签，命中的文章聚合成独立话题板块，跨天延续成一条话题生命线；从下一期日报开始生效。
        </p>
      </div>

      <!-- keyword_topic 态：三件套（语法提示 + 实时解析预览 + 物化预期） -->
      <div v-else class="twcd-field">
        <label class="twcd-field__label" for="twcd-kw-input">关键字表达式</label>
        <AppInput
          id="twcd-kw-input"
          v-model="keywordText"
          placeholder="如：ASML|镓锗 出口"
          data-testid="watch-keyword-input"
          @keydown.enter="submit"
        />
        <p class="twcd-kw-hint">
          <b>空格</b> = 且（全含才命中）· <code>|</code> = 或（含任一即命中）· 大小写不敏感。
          匹配范围：当天文章标题 + 摘要。
        </p>

        <!-- 实时解析预览：chips -->
        <div class="twcd-preview" data-testid="keyword-parse-preview" :aria-live="keywordTouched ? 'polite' : 'off'">
          <template v-if="keywordValid && keywordSlots.length">
            <template v-for="(slot, i) in keywordSlots" :key="i">
              <span v-if="i > 0" class="twcd-preview__and" aria-label="且">×</span>
              <span class="twcd-preview__slot" data-testid="keyword-parse-slot">
                <template v-for="(alt, j) in slot" :key="j">
                  <span v-if="j > 0" class="twcd-preview__or" aria-label="或">|</span><span>{{ alt }}</span>
                </template>
              </span>
            </template>
          </template>
          <p v-else class="twcd-preview__invalid" data-testid="keyword-parse-invalid">
            {{ keywordText.trim() ? '未解析出有效关键字，提交将被拒绝' : '输入表达式后这里会实时预览解析结果' }}
          </p>
        </div>

        <!-- 物化预期说明 -->
        <p class="twcd-scan-note">
          <Icon icon="mdi:text-box-search" width="14" aria-hidden="true" />
          每期日报把当天含关键字的全部文章（含未打标签的漏网文章）聚合成固定名称的独立板块；从下一期日报开始生效。
        </p>
      </div>

      <p v-if="submitError" class="twcd-error" role="alert" data-testid="watch-create-error">{{ submitError }}</p>
    </div>

    <template #footer>
      <AppButton variant="ghost" size="sm" data-testid="watch-create-cancel" @click="close">
        取消
      </AppButton>
      <AppButton
        variant="primary"
        size="sm"
        :loading="submitting"
        :disabled="!canSubmit"
        data-testid="watch-create-submit"
        @click="submit"
      >
        创建关注
      </AppButton>
    </template>
  </AppDialog>
</template>

<style scoped>
.twcd {
  display: flex;
  flex-direction: column;
  gap: 0.8rem;
}

/* —— 类型四选卡（2×2） —— */
.twcd-type {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 0.5rem;
}

.twcd-type__card {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.15rem;
  padding: 0.6rem 0.4rem;
  border: 1px solid var(--color-border-medium);
  border-radius: 8px;
  background: transparent;
  color: var(--color-text-secondary);
  font-family: inherit;
  cursor: pointer;
  transition: border-color 0.15s, color 0.15s, background 0.15s;
}

.twcd-type__card:hover {
  border-color: var(--color-border-strong);
  color: var(--color-text-primary);
}

.twcd-type__card.is-on {
  border-color: var(--color-accent);
  background: var(--color-accent-subtle);
  color: var(--color-accent);
}

.twcd-type__name {
  font-size: 0.8rem;
  font-weight: 600;
}

.twcd-type__desc {
  font-size: 0.64rem;
  color: var(--color-text-muted);
}

.twcd-type__card.is-on .twcd-type__desc {
  color: var(--color-accent);
}

/* —— 输入区 —— */
.twcd-field {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
}

.twcd-field__label--second {
  margin-top: 0.6rem;
}

.twcd-field__label {
  font-size: 0.72rem;
  color: var(--color-text-secondary);
}

.twcd-field__hint {
  margin: 0;
  font-size: 0.68rem;
  color: var(--color-text-muted);
  line-height: 1.6;
}

/* —— keyword 语法提示 —— */
.twcd-kw-hint {
  margin: 0;
  font-size: 0.68rem;
  color: var(--color-text-muted);
  line-height: 1.7;
}

.twcd-kw-hint b {
  color: var(--color-text-secondary);
  font-weight: 600;
}

.twcd-kw-hint code {
  font-family: ui-monospace, SFMono-Regular, Consolas, monospace;
  font-size: 0.66rem;
  background: var(--color-bg-sunken);
  border-radius: 3px;
  padding: 0.05rem 0.3rem;
}

/* —— 解析预览 chips —— */
.twcd-preview {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 0.35rem;
  min-height: 1.6rem;
}

.twcd-preview__slot {
  display: inline-flex;
  align-items: center;
  font-family: ui-monospace, SFMono-Regular, Consolas, monospace;
  font-size: 0.7rem;
  color: var(--color-tag-keyword);
  background: var(--color-tag-keyword-bg);
  border: 1px dashed var(--color-tag-keyword-border);
  border-radius: 5px;
  padding: 0.18rem 0.5rem;
}

.twcd-preview__or {
  margin: 0 0.15rem;
  color: var(--color-text-muted);
}

.twcd-preview__and {
  font-size: 0.66rem;
  color: var(--color-text-muted);
}

.twcd-preview__invalid {
  margin: 0;
  font-size: 0.7rem;
  font-style: italic;
  color: var(--color-error);
}

/* —— 回扫预期说明 —— */
.twcd-scan-note {
  display: flex;
  align-items: flex-start;
  gap: 0.4rem;
  margin: 0.15rem 0 0;
  font-size: 0.7rem;
  color: var(--color-text-secondary);
  line-height: 1.6;
  background: var(--color-bg-sunken);
  border-radius: 6px;
  padding: 0.5rem 0.65rem;
}

.twcd-scan-note svg {
  flex: none;
  margin-top: 0.15rem;
  color: var(--color-success);
}

/* —— 错误提示 —— */
.twcd-error {
  margin: 0;
  color: var(--color-error);
  font-size: 0.76rem;
}
</style>
