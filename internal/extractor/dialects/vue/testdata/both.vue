<script lang="ts">
export const version = '1.0'
export default { inheritAttrs: false }
</script>

<script setup lang="ts">
import * as Tabs from './tabs'
import Panel from '../Panel.vue'
const emit = defineEmits<{ (e: 'change', v: string): void }>()
const onChange = (v: string) => emit('change', v)
</script>

<template>
  <Tabs.Root>
    <Tabs.Item v-for="t in tabs" :key="t" />
  </Tabs.Root>
  <Panel v-if="open" />
  <div v-pre>{{ raw }}</div>
</template>
