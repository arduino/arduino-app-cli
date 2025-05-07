<script>
  import { onMount } from 'svelte';
  import { MkTempDir,PullSync,ListFiles } from '../wailsjs/go/main/App.js'

  let files = []
  onMount(async () => {
    const tmpPath = await MkTempDir("apps")
    console.log("tmpPath", tmpPath)
    await PullSync(tmpPath, ".")
    files = await ListFiles(tmpPath)
    console.log("files", files)
  })
</script>

<main>
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
