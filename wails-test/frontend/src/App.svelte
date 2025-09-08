<script>
  import { onMount } from 'svelte';
  import { GetVersion, CheckAndApplyUpdate, GetLatestVersion } from '../wailsjs/go/main/App.js'

  let version = '';
  let latestVersion = '';
  let updateError = '';

  onMount(async () => {
    version = await GetVersion();
    latestVersion = await GetLatestVersion();
  })

  async function checkForUpdates() {
    updateError = '';
    try {
      await CheckAndApplyUpdate();

    } catch (error) {
      updateError = error?.message || error;
      console.error("Error checking for updates:", error);
    }
  }

</script>

<main>
  <p>Current version: {version}</p>

  <p>Latest version: {latestVersion} </p>

  <button on:click={checkForUpdates}>Check For Updates</button>

  {#if updateError}
    <p style="color: red;">Error: {updateError}</p>
  {/if}
</main>
