import { addCollection } from '@iconify/vue'
import subset from '~/assets/iconify-subset.json'

// Register the locally generated mdi subset once at startup so the <Icon>
// component resolves icons from local data instead of fetching them from
// api.iconify.design at runtime — icons keep rendering when offline or behind
// a proxy that blocks that domain.
//
// The subset is produced from the icons actually referenced in app/ source;
// regenerate it after adding new icons:
//   pnpm generate:icons
export default defineNuxtPlugin(() => {
  addCollection(subset)
})
