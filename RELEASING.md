# Releasing

Needs macOS, Go from `go.mod`, an authenticated GitHub CLI, the
`Developer ID Application: ROBOPUNK FOUNDATION (68S3R4UWP6)` identity in
the login keychain, and the notary credentials stored there as
`robopunk-notary` (`xcrun notarytool store-credentials robopunk-notary`).

1. Set `version` in `mygo.json`, run `go test ./...` and the end-to-end
   and real-window checks of the README, and commit.
2. Build, sign and notarize a universal app and disk image, and the
   Linux and Windows packages (the Windows installers need
   `brew install makensis`):

   ```sh
   rm -rf build && go tool mygo build -platform darwin/universal,linux/amd64,linux/arm64,windows/amd64,windows/arm64
   ```

   The Windows installers are not signed: no Windows certificate is set
   up, so SmartScreen warns before they run.

3. Check the result before publishing:

   ```sh
   cd build/darwin-universal
   spctl -a -vv -t open --context context:primary-signature mattermost-go-VERSION.dmg
   xcrun stapler validate mattermost-go-VERSION.dmg
   lipo -archs "Mattermost Go.app/Contents/MacOS/Mattermost Go"   # arm64 x86_64
   shasum -a 256 mattermost-go-VERSION.dmg > mattermost-go-VERSION.dmg.sha256
   cd .. && shasum -a 256 */*.deb */*.rpm */*.tar.gz */*setup*.exe darwin-universal/*.dmg > SHA256SUMS
   ```

4. Tag, push and publish:

   ```sh
   git tag vVERSION && git push origin main vVERSION
   gh release create vVERSION build/darwin-universal/mattermost-go-VERSION.dmg \
     build/darwin-universal/mattermost-go-VERSION.dmg.sha256 build/linux-*/*.deb build/linux-*/*.rpm \
     build/linux-*/*.tar.gz build/windows-*/*setup*.exe build/SHA256SUMS \
     --title "Mattermost Go VERSION" --notes-file NOTES.md
   ```

5. Update `Casks/mattermost-go.rb` in `txchen/homebrew-tap` with the
   version and the dmg's SHA-256, push it, and check the install:

   ```sh
   brew update && brew install --cask txchen/tap/mattermost-go
   spctl -a -vv "/Applications/Mattermost Go.app"
   ```
