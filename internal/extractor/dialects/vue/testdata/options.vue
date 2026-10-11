<template>
  <section>
    <router-link to="/">home</router-link>
    <my-card :title="title" @close="close">
      <p>{{ title }}</p>
    </my-card>
    <base-transition />
  </section>
</template>

<script>
import MyCard from './MyCard.vue'
import { format } from './util.js'

export default {
  name: 'Options',
  components: { MyCard },
  data() {
    return { title: format('x') }
  },
  methods: {
    close() {
      this.$emit('close')
    },
  },
}
</script>

<style>
section { margin: 0; }
</style>

<i18n lang="json">
{ "en": { "hello": "<Fake/>" } }
</i18n>

<docs>
Use it as <OptionsDemo />.
</docs>
