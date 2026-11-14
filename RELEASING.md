# Releasing

Needs macOS, Go from `go.mod`, an authenticated GitHub CLI, the
`Developer ID Application: ROBOPUNK FOUNDATION (68S3R4UWP6)` identity in
the login keychain, and the notary credentials stored there as
`robopunk-notary` (`xcrun notarytool store-credentials robopunk-notary`).

1. Set `version` in `mygo.json`, run `go test ./...` and the end-to-end
   and real-window checks of the README, and commit.
2. Build, sign and notarize every platform, with the signed updates (the
   Windows installers need `brew install makensis`; the update key is in
   `~/Library/Application Support/mygo/update-keys`, kept out of the
   repository, with a copy in the password manager):

   ```sh
   rm -rf build && MYGO_UPDATER_PRIVATE_KEY="$(cat ~/Library/Application\ Support/mygo/update-keys/mygo-update.key)" \
     go tool mygo build -platform darwin/universal,linux/amd64,linux/arm64,windows/amd64,windows/arm64
   ```

   The Windows installers are not signed: no Windows certificate is set
   up, so SmartScreen warns before they run. Write the version's section
   of `CHANGELOG.md` first: the update window shows it.

3. Check the result before publishing:

   ```sh
   cd build/darwin-universal
   spctl -a -vv -t open --context context:primary-signature mattermost-go-VERSION.dmg
   xcrun stapler validate mattermost-go-VERSION.dmg
   lipo -archs "Mattermost Go.app/Contents/MacOS/Mattermost Go"   # arm64 x86_64
   shasum -a 256 mattermost-go-VERSION.dmg > mattermost-go-VERSION.dmg.sha256
   ```

4. Tag, push and publish:

   ```sh
   git tag vVERSION && git push origin main vVERSION
   gh release create vVERSION build/darwin-universal/mattermost-go-VERSION.dmg \
     build/darwin-universal/mattermost-go-VERSION.dmg.sha256 build/linux-*/*.deb build/linux-*/*.rpm \
     build/darwin-universal/*.tar.gz build/darwin-universal/update-*.json \
     build/linux-*/*.tar.gz build/linux-*/update-*.json build/linux-amd64/install.sh \
     build/windows-*/*setup*.exe build/windows-*/*.tar.gz build/windows-*/update-*.json \
     build/windows-amd64/install.ps1 build/SHA256SUMS \
     --title "Mattermost Go VERSION" --notes-file NOTES.md
   ```

   Installed apps read `update-<platform>.json` of the latest release and
   update to it. `install.sh` is the same file for every platform.

5. Update `Casks/mattermost-go.rb` in `txchen/homebrew-tap` with the
   version and the dmg's SHA-256, so that new installs get it, push it,
   and check the install. The cask says `auto_updates true`, so
   `brew upgrade` leaves installed apps to update themselves:

   ```sh
   brew update && brew install --cask txchen/tap/mattermost-go
   spctl -a -vv "/Applications/Mattermost Go.app"
   ```
