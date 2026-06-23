/**
 * ChapterTransition — board-switch wipe + classified-cover sequence.
 *
 * Phase 1: red wipe sweeps left→right (0–0.15s).
 * Phase 2: archive cover fades in + typewriter title + CONFIDENTIAL stamp (0.15–0.65s).
 * Phase 3: cover fades out, scene reloads, camera flies in, cards stagger up (0.65–1.0s).
 *
 * Vue renders the DOM (overlay container); this class only animates styles.
 * Typewriter uses substring + onUpdate (no GSAP TextPlugin dependency).
 *
 * @see specs/detective-wall-camera/spec.md §Chapter Transition
 */
import gsap from 'gsap'

export interface ChapterTransitionData {
  name: string
  dateRange: string
  topicCount: number
  categoryBreakdown?: { label: string; count: number }[]
}

export interface ChapterTransitionHooks {
  /** Called during Phase 3 to reload the scene with the new board's data. */
  onReload: () => void
}

export class ChapterTransition {
  isPlaying = false
  private activeTimeline: gsap.core.Timeline | null = null

  constructor(
    private readonly wipeEl: HTMLElement,
    private readonly coverEl: HTMLElement,
    private readonly titleEl: HTMLElement,
    private readonly hooks: ChapterTransitionHooks,
  ) {}

  play(data: ChapterTransitionData): gsap.core.Timeline {
    if (this.activeTimeline) this.activeTimeline.kill()
    this.isPlaying = true

    const tl = gsap.timeline({
      onComplete: () => {
        this.isPlaying = false
        this.resetDom()
      },
    })
    this.activeTimeline = tl

    // --- Phase 1: red wipe ---
    tl.set(this.wipeEl, { display: 'block', xPercent: -100 })
      .to(this.wipeEl, { xPercent: 100, duration: 0.15, ease: 'power2.in' })

    // --- Phase 2: archive cover + typewriter ---
    const fullTitle = data.name
    tl.set(this.coverEl, { display: 'flex', opacity: 0, scale: 0.95 })
      .to(this.coverEl, { opacity: 1, scale: 1, duration: 0.15, ease: 'back.out(1.2)' })
      // Typewriter via substring proxy.
      .fromTo(
        { progress: 0 },
        { progress: 0 },
        {
          progress: 1,
          duration: Math.min(0.3, fullTitle.length * 0.03),
          ease: 'none',
          onUpdate: function () {
            const prog = this.targets()[0].progress as number
            titleElSubstring(data.name, prog)
          },
        },
      )
      .to({}, { duration: 0.2 }) // hold to read

    // --- Phase 3: reload + fly-in ---
    tl.to(this.coverEl, { opacity: 0, scale: 1.02, duration: 0.15 })
      .add(() => this.hooks.onReload())
      .to(this.wipeEl, { display: 'none', duration: 0 })

    return tl
  }

  kill(): void {
    if (this.activeTimeline) {
      this.activeTimeline.kill()
      this.activeTimeline = null
    }
    this.isPlaying = false
    this.resetDom()
  }

  private resetDom(): void {
    this.wipeEl.style.display = 'none'
    this.coverEl.style.display = 'none'
    this.coverEl.style.opacity = '0'
    if (this.titleEl) this.titleEl.textContent = ''
  }
}

/** Write the typewriter substring into the title element. */
function titleElSubstring(full: string, progress: number): void {
  const el = document.querySelector<HTMLElement>('[data-chapter-title]')
  if (!el) return
  const count = Math.ceil(full.length * Math.max(0, Math.min(1, progress)))
  el.textContent = full.slice(0, count)
}
