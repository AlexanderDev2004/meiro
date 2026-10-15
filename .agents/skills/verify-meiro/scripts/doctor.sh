#!/usr/bin/env bash
# Read-only check that this checkout can run a proof. Exit 1 on a blocker.
set -u
cd "$(git rev-parse --show-toplevel)" || exit 1
fail=0
ok() { printf 'ok    %s\n' "$1"; }
bad() { printf 'FAIL  %s\n' "$1"; fail=1; }
note() { printf 'note  %s\n' "$1"; }

want=$(awk '/^go /{print $2}' go.mod)
have=$(go env GOVERSION 2>/dev/null | sed 's/^go//')
if [ -n "$have" ] && [ "$(printf '%s\n%s\n' "$want" "$have" | sort -V | head -1)" = "$want" ]; then
	ok "go $have (go.mod wants $want)"
else
	bad "go ${have:-missing}, go.mod wants $want"
fi

mygo=$(go list -m -f '{{.Dir}}' github.com/egoist/mygo 2>/dev/null)
if [ -n "$mygo" ] && [ -d "$mygo" ]; then ok "mygo source at $mygo"; else bad "mygo is not in the module cache: run go mod download"; fi

if command -v ffmpeg >/dev/null; then ok "ffmpeg $(ffmpeg -version | head -1 | cut -d' ' -f3)"; else bad "ffmpeg missing: playback proofs need it"; fi
command -v yt-dlp >/dev/null && ok "yt-dlp present" || note "yt-dlp missing: fine, tests replace the resolver"
command -v bazel >/dev/null && ok "bazel present (lint, format, race)" || note "bazel missing: use go test, skip lint and format"

if [ -f "$HOME/.cache/amp/desktop/environment" ]; then
	# shellcheck disable=SC1091
	. "$HOME/.cache/amp/desktop/environment"
	if [ -S "$XDG_RUNTIME_DIR/$WAYLAND_DISPLAY" ] && grim - >/dev/null 2>&1; then
		ok "Desktop is up: the native window can open (scripts/native.sh)"
		for t in grim wtype wlrctl yt-dlp; do
			command -v "$t" >/dev/null && ok "$t" || bad "$t missing: run .agents/setup"
		done
	else
		note "Desktop is not running: run 'amp orb desktop ensure' before scripts/native.sh"
	fi
else
	note "no Desktop: native proofs unavailable, headless proofs still work"
fi

if [ -n "$(git status --porcelain -- . ':!.gitignore')" ]; then
	note "working tree has changes: this is the 'after' state"
fi
ls zz_proof*_test.go >/dev/null 2>&1 && note "scratch proofs present: $(ls zz_proof*_test.go | tr '\n' ' ')delete them before committing"

if go vet ./youtube/ >/dev/null 2>&1; then ok "go vet ./youtube builds"; else bad "go vet ./youtube failed: the module does not compile"; fi
exit $fail
