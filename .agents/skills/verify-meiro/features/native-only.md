# Native-only behavior

These behaviors need a real window, device, account or network. The headless tester cannot show them. The orb Desktop covers some of them (see below). A change that touches one reports it as unverified unless it was run where the dependency exists.

## Sub-features

- `native-window` covers the OS window, title bar, vibrancy, tray, and the mini player window.
- `native-audio` covers the real sound card, device selection, and volume.
- `native-media-keys` covers MPRIS on Linux and Now Playing on macOS (`systemmedia/`).
- `native-youtube` covers real InnerTube responses, real sign-in, and region or age blocks.
- `native-ytdlp` covers resolving real audio URLs and reading browser cookies.
- `native-deeplink` covers `meiro://` links and single instance behavior.
- `native-update` covers the updater and release packaging.

## How to get to it (user POV)

- In an Amp orb, run `scripts/native.sh start`. It needs the Desktop (`amp orb desktop ensure`) and the tools `.agents/setup` installs.
- On a laptop, run `bazel run //:dev` (live reload) with `ffmpeg` and `yt-dlp` on `PATH`.
- Install a release build from `scripts/package-linux.sh` or the release workflow.

## Driving it with the headless tester

Preconditions:

- `scripts/doctor.sh` reports `Desktop is up` when `scripts/native.sh` can run.

- **Verified on the orb Desktop.** `native-window` (a real labwc window, fullscreen), `native-audio` (a `Meiro` stream on the Desktop's null sink with the position advancing), `native-youtube` signed out (live home, search, playlists), and `native-ytdlp` resolving a real stream. Search for a name, click a result, read the screenshot and `pactl list sink-inputs`.
- **Not verifiable there.** Sign-in with a real account, MPRIS and macOS Now Playing, deep links on another OS, the updater, macOS and Windows rendering, a real sound card.

- **What does run.** `systemmedia` has its own unit tests. `go test ./systemmedia/ ./player/ ./internal/...` exercises them without a device.
- **Cross-build.** `bazel build --config=darwin-arm64 //...` and the other `--config` targets in `.bazelrc` type-check a platform you cannot run.
- **Everything else.** Write a short checklist in the PR, with the OS, what you ran and what you saw. Do not claim it from the headless proof.

## Gotchas

- The headless window has no title bar and draws with the software renderer. Layout is real. GPU, DPI and font fallback on other platforms are not.
- Never use a personal YouTube account or cookies in a proof, or put them in an artifact.
- The Desktop output is software rendered and its audio is a null sink. Nobody hears it. The tray icon is unavailable there, and the app logs that and goes on.
- Do not change the Desktop's output mode. It crashes the stream the user watches.
