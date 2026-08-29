<template>
  <label class="relative inline-flex items-center gap-1.5 text-muted-foreground" :title="t('common.language')">
    <GlobeOutline class="size-4 shrink-0" />
    <span class="sr-only">{{ t('common.language') }}</span>
    <select
      :value="locale"
      class="h-9 max-w-[9.5rem] appearance-none rounded-xl border-2 border-ac-sand bg-card pl-2 pr-7 text-xs font-bold text-foreground outline-none transition-colors hover:border-primary/50 focus:border-primary"
      @change="changeLocale"
    >
      <option v-for="option in localeOptions" :key="option.value" :value="option.value">
        {{ option.label }}
      </option>
    </select>
    <ChevronDownOutline class="pointer-events-none absolute right-2 size-3" />
  </label>
</template>

<script setup>
import { useI18n } from 'vue-i18n'
import { GlobeOutline, ChevronDownOutline } from '@vicons/ionicons5'
import { localeOptions, setLocale } from '../../i18n'

const { t, locale } = useI18n({ useScope: 'global' })

function changeLocale(event) {
  setLocale(event.target.value)
}
</script>
