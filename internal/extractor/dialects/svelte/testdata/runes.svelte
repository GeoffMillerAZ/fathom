<script module lang="ts">
  export const prerender = true;
</script>

<script lang="ts">
  import { onMount } from 'svelte';
  import Button from './Button.svelte';
  import * as Card from '$lib/components/ui/card/index.js';
  import { cn } from '../utils.js';
  let { data, children } = $props();
  let count = $state(0);
  const doubled = $derived(count * 2);
  function inc() {
    count += 1;
  }
  onMount(() => load());
</script>

<svelte:head><title>x</title></svelte:head>
<div class={cn('a')} onclick={inc}>
  <Button variant="ghost">{count}</Button>
  <Card.Root>
    <Card.Content />
  </Card.Root>
  {#if data.ok}
    <Icon />
  {:else}
    <svelte:component this={data.dyn} />
  {/if}
  {#snippet row(item)}<Row {item} />{/snippet}
  {@render children?.()}
  <input bind:value={count} />
</div>

<style>
  .a { color: red; }
</style>
