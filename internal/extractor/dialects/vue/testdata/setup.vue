<script setup lang="ts">
import { ref, onMounted } from 'vue'
import ElButton from './button.vue'
import { useNamespace } from '../hooks/use-namespace'
import Icon from '@/components/Icon.vue'

interface Props {
  size?: string
}
const props = defineProps<Props>()
const count = ref(0)
const ns = useNamespace('button')
function increment(step: number) {
  count.value += step
}
const reset = () => {
  count.value = 0
}
onMounted(() => {
  increment(1)
})
defineExpose({ reset })
</script>

<template>
  <div :class="ns.b()">
    <ElButton @click="increment(1)">{{ count }}</ElButton>
    <el-input v-model="text" />
    <transition name="fade">
      <keep-alive><RouterView /></keep-alive>
    </transition>
    <component :is="dyn" />
    <slot name="footer" />
    <svg><path d="M0 0" /></svg>
    <template #header><Icon /></template>
  </div>
</template>

<style scoped lang="scss">
.btn { color: red; }
</style>
