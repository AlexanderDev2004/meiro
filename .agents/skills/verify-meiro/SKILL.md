---
name: verify-meiro
description: Proves a Meiro change works end to end by driving the real app headlessly with screenshots, step texts, request counters and before/after runs. Use when adding or changing a feature or fix in this repo, when asked to prove, demo or test it end to end, or before opening a PR that changes what a user sees.
---

# Verify Meiro

Meiro is a native desktop music app (MyGo UI, Go). No display is needed to prove a change. `ui.NewTester` runs the real app and draws real pixels, `fakeMusic` stands in for YouTube, and playback decodes real audio with ffmpeg on a null device. A proof is a scratch Go test that drives the app by clicking labels and records a screenshot for each step.

Read `AGENTS.md` and `../mygo-maintenance/SKILL.md` for the code rules. This skill is for the evidence.

## When you add a feature

1. Open `features/README.md`. Find the feature file your change touches, or add one (same four H2s as the others) and a line in the index.
2. Run `scripts/doctor.sh`.
3. Copy `templates/zz_proof_test.go.tmpl` to `zz_proof_<feature>_test.go` in the repo root and write the scenario from the feature file's recipe.
4. Run `scripts/prove.sh --before base --label <feature> 'TestProof'`. Read the screenshots. Fix the app or the scenario until the after run passes for the right reason.
5. Keep the part that users or other code rely on as a normal test in the existing `*_test.go` files, per `AGENTS.md`. Delete the scratch proof. Update the feature file with what you learned.
6. Add a `CHANGELOG.md` entry when the change is user visible.
7. Report with the evidence under Evidence below.

## Doctor

`scripts/doctor.sh` is read-only. It checks the Go version against `go.mod`, the MyGo module in the module cache, ffmpeg, that `./youtube` compiles, and whether the Desktop and its tools (grim, wtype, wlrctl, yt-dlp) are ready. Run it first, and again when a proof behaves oddly. A `FAIL` line blocks proofs. A `note` does not.

## Launch

There is no process to start. Each proof builds a fresh app in memory and a window with `ui.NewTester(a.view, 1000, 700)`. State starts empty, with settings and history in a temp dir where the scenario sets one.

Use the constructors that exist.

- `newTestApp()` is a signed-out app on the fake YouTube.
- `signedInApp(t)` has a saved session. Call `resetFakes()` before it.
- `playbackApp(t)` resolves streams through a fake resolver `r`. Set `r.file` for real audio.
- `stateApp(t, s, signedIn)` lets a test switch YouTube to offline, 500 or empty.

## Drive

Click what the user clicks, by visible label. `p.click(label, step)` clicks and records. `p.openMenu(title, step)` hovers a row and opens its menu. `tt.Type`, `tt.Key`, `tt.RightClick` and `tt.Scroll` do the rest. Do not call `a.play`, `a.enqueue` and the like to act. Read them to assert.

Labels, seed titles and recipes for each feature are in `features/`. Existing tests are the best source of exact steps. Search them for the label you need (`grep -n 'Click("' *_test.go`).

A feature that needs a response the fake does not have gets a new canned response in `harness_test.go`, in the same change. Keep the fake honest. A fake that cannot fail cannot prove failure handling, so also cover the failing case with `refuseWrites`, `switchable.mode` or `r.err`.

## Evidence

A proof the reader can trust has all of these.

- The action and the result. One screenshot per step from `p.step`, not only the last frame. Look at the images. A capture you did not read verifies nothing.
- A second view of every side effect. For a write, assert the counter and `written()`. For settings or history, read the file. For playback, read `a.player.Position()` after real audio and `a.history`.
- Before and after. `scripts/prove.sh --before base` runs the same scratch tests on a clean worktree of the merge base. For a fix, before must fail for the reason the bug report gives. For a feature, before fails because the feature is absent. Say so, and show both screenshots side by side.
- A control. One case that should not change, such as the signed-out app that must show no like button, or the page that stays unchanged.
- The full suite. `bazel test //...` and `bazel run //:lint` when Bazel works, otherwise `go test ./... -count=1` and `go vet ./...`. Say which ran. Add `--race` to `prove.sh` for threading changes.
- What stays unproven. `features/native-only.md` lists it. Name each item your change touches.

Artifacts are in `.amp/in/artifacts/verify-meiro/<label>/{before,after}/` with `NN-step.png`, `steps.txt` (every text on screen per step), `go-test.log` and `exit-code`. The directory is gitignored. Embed the screenshots that matter in the final answer.

## Cleanup

Delete `zz_proof*_test.go` from the repo root. `.gitignore` already ignores it, but a stray file still breaks `bazel test` if listed and confuses the next agent. `prove.sh` removes its own worktree. It never removes artifacts. Do not delete the artifacts directory.

## Native app

The orb Desktop (labwc, Wayland, a null audio sink) runs the real window against live YouTube Music, signed out. Use it when a change touches what the headless tester cannot see, and for one final look at any user visible change. `.agents/setup` installs grim, wtype, wlrctl and yt-dlp.

```sh
scripts/native.sh start          # builds, starts the Desktop, launches Meiro on empty data, fullscreen
scripts/native.sh shot home      # read the PNG, then pick pixels from it
scripts/native.sh click 980 45   # pixels of the last screenshot
scripts/native.sh type yorushika
scripts/native.sh key Return     # also ctrl+k, Tab, space
scripts/native.sh stop
```

Rules learned the hard way.

- Take a screenshot before every click and read the coordinates from it. Layout moves between runs.
- Never change the output mode or scale (`wlr-randr`). It crashes the Desktop tab's stream. If that happens, kill the `labwc` process and run `amp orb desktop ensure`.
- Wait after each action. `native.sh` already sleeps 0.5 to 0.7 s per input. Add a sleep for network results.
- Prove sound by the stream, not by ear. `. ~/.cache/amp/desktop/environment; pactl list sink-inputs` shows a `Meiro` input with `Corked: no`, and the player position in the screenshot advances.
- Meiro runs with `PATH` including `~/.local/bin` (for yt-dlp) and fresh `XDG_*` dirs under `.amp/in/native/data`. Its log is `.amp/in/native/data/config/meiro/logs/meiro.log`.
- Never sign in with a real account here. The artifacts hold everything on screen.
- Real YouTube changes. Do not assert on seeded titles. Assert on structure and on behavior.

A user can watch the same window in the Desktop tab. `features/native-only.md` lists what still needs another OS.

## Helpers

- `scripts/doctor.sh` prints `ok`, `note` or `FAIL` lines and exits 1 on a blocker.
- `scripts/prove.sh [--before REF|base] [--label NAME] [--race] TEST_REGEX` runs scratch tests with `PROOF_DIR` set and prints the pass or fail lines, the exit code and the artifact path. Exit status is the after run's status.
- `scripts/native.sh start|shot|click|rclick|type|key|status|stop` drives the real window on the Desktop. Screenshots go to `.amp/in/artifacts/verify-meiro/native/`.
- `templates/zz_proof_test.go.tmpl` defines `proof` (`step`, `click`, `openMenu`, `shows`) and one working example scenario, `TestProofExample`.
