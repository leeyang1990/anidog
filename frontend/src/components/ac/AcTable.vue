<template>
  <div class="ac-table-wrap overflow-x-auto rounded-3xl border-2 border-ac-sand bg-card">
    <table class="w-full text-sm">
      <thead>
        <tr class="bg-ac-sand/50 text-left">
          <th
            v-for="col in columns"
            :key="col.key"
            class="px-4 py-3 font-bold text-foreground tracking-tight whitespace-nowrap"
            :class="[col.align === 'right' ? 'text-right' : col.align === 'center' ? 'text-center' : 'text-left', col.headerClass]"
            :style="col.width ? { width: col.width } : undefined"
          >
            {{ col.title }}
          </th>
        </tr>
      </thead>
      <tbody>
        <tr
          v-for="(row, i) in data"
          :key="rowKey ? row[rowKey] : i"
          class="border-t-2 border-dashed border-ac-sand transition-colors hover:bg-ac-cream/60"
        >
          <td
            v-for="col in columns"
            :key="col.key"
            class="px-4 py-3 align-middle"
            :class="[col.align === 'right' ? 'text-right' : col.align === 'center' ? 'text-center' : 'text-left', col.cellClass]"
          >
            <slot :name="`cell-${col.key}`" :row="row" :index="i" :value="row[col.key]">
              {{ row[col.key] ?? '—' }}
            </slot>
          </td>
        </tr>
        <tr v-if="!data.length">
          <td :colspan="columns.length" class="px-4 py-12 text-center text-muted-foreground text-sm">
            <slot name="empty">什么也没有~</slot>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<script setup>
defineProps({
  columns: { type: Array, default: () => [] }, // [{key,title,align,width,headerClass,cellClass}]
  data: { type: Array, default: () => [] },
  rowKey: { type: String, default: '' },
})
</script>
