<script>
  import Item from './Item.svelte';
  import { createEventDispatcher } from 'svelte';
  export let items = [];
  const dispatch = createEventDispatcher();
  $: total = items.length;
  function pick(i) {
    dispatch('pick', i);
  }
</script>

<ul>
  {#each items as item, i}
    <Item {item} on:click={() => pick(i)} />
  {/each}
</ul>
{#await promise then value}
  <svelte:self items={value} />
{/await}
<slot />
