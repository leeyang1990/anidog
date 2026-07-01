<template>
  <span
    class="ac-tag inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-semibold border-2 leading-none whitespace-nowrap"
    :class="variantCls"
  >
    <slot name="icon" />
    <slot />
    <button
      v-if="closable"
      type="button"
      class="ml-1 opacity-70 hover:opacity-100 transition-opacity"
      @click="$emit('close')"
    >×</button>
  </span>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  variant: { type: String, default: 'default' }, // default | grass | sun | sky | heart | leaf | wood
  closable: { type: Boolean, default: false },
})

defineEmits(['close'])

const variantCls = computed(() => ({
  default: 'bg-ac-sand text-ac-earth border-ac-sand-dark',
  grass: 'bg-ac-grass-light/40 text-ac-grass-dark border-ac-grass-light',
  sun: 'bg-ac-sun/30 text-[#A35400] border-ac-sun',
  sky: 'bg-ac-sky/40 text-ac-sky-dark border-ac-sky',
  heart: 'bg-ac-heart/25 text-ac-heart-dark border-ac-heart',
  leaf: 'bg-ac-leaf/25 text-ac-leaf-dark border-ac-leaf',
  wood: 'bg-ac-wood/20 text-ac-wood-dark border-ac-wood',
}[props.variant] || 'bg-ac-sand text-ac-earth border-ac-sand-dark'))
</script>
