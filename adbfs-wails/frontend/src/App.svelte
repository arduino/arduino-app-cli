<script>
  import { onMount } from 'svelte';
  import { MkTempDir,PullSync,ListFiles, GetVersion, CheckAndApplyUpdate, GetLatestVersion } from '../wailsjs/go/main/App.js'

  let files = []
  let version = '';
  let latestVersion = '';
   let updateError = '';

  onMount(async () => {
    version = await GetVersion();
    latestVersion = await GetLatestVersion();

    const tmpPath = await MkTempDir("apps")
    console.log("tmpPath", tmpPath)
    await PullSync(tmpPath, ".")
    files = await ListFiles(tmpPath)
    console.log("files", files)
  })

  async function checkAndApplyUpdate() {
    try{
      console.log("Checking for updates...")
      await CheckAndApplyUpdate()
    } catch (error) {
        updateError = error?.message || error;
      console.error("Error checking for updates:", error);
    }
  }

</script>

<main>
  <p>Current version: {version}</p>

  <p>Latest version: {latestVersion} </p>

  <button on:click={checkAndApplyUpdate}>Update</button>

  {#if updateError}
    <p style="color: red;">Error: {updateError}</p>
  {/if}

  <div>list of files with adb</div>

  <br/>
  <div>
      {#each files as file }
          <div>
              {file.IsDir ? '📁' : '📄'} {file.Path}
          </div>
      {/each}
  </div>
</main>

<style>
</style>
