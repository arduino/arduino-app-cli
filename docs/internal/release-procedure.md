# Release Procedure


## Steps

The following are the steps to follow to make a release of Arduino App CLI:

### 1. Create the pre-release on GitHub

You need to **create and push the new tag** and wait for the pre-release to appear on [the "**Releases**" page](https://github.com/arduino/arduino-app-cli/releases).


1. Checkout the release branch in the repository.
1. Run the following commands:
   ```text
   git pull
   git tag -a <YOUR_VERSION> -m "<YOUR_VERSION>"
   git push origin <YOUR_VERSION>
   ```

Pushing a tag will trigger a **GitHub Actions** workflow on the `main` branch. Check the "**Arduino App CLI**" workflow and see that everything goes right. If the workflow succeeds, a new pre-release will be created automatically and you should see it on the ["**Releases**"](https://github.com/arduino/arduino-app-cli/releases) page.

### 2. ✅ Validate the pre-relaese

- Ask in the  #ft_swc_qa_requests Slack channel to perform QA testing
- Write a message in the #general Slack channel:


### 3. 🚢 Create the release on GitHub

### 4. 📄 Create the changelog

Edit the `CHANGELOG.md` file following the **Keep A Changelog** scheme:

https://keepachangelog.com/en/1.0.0/#how

Add a list of mentions of GitHub users who contributed to the release in any of the following ways (ask @per1234):

- Submitted a PR that was merged
- Made a valuable review of a PR
- Submitted an issue that was resolved
- Provided valuable assistance with the investigation of an issue that was resolved

Add a "**Known Issues**" section at the bottom of the changelog.



### 3. 😎 Brag about it
- Write a message in the `#ft_swc_deploy` **Slack** channel:
  > Hey **Arduino**s! Updates from the **Tooling Team** :hammer_and_wrench:
  >
  > Arduino App CLU 0.6.8 is out! :doge: You can download it from the [Download Page](https://github.com/arduino/arduino-app-cli/releases)
  > The highlights of this release are:
  >
  > - add the ardino-app-cli board monitor command
  > - improvement of Serial Monitor performances
  > - ardiuno-cli upgrade
  > - some bugfixing
  >
  > To see the details, you can take a look at the [Changelog](https://github.com/arduino/arduino-app-cli/blob/main/CHANGELOG.md)