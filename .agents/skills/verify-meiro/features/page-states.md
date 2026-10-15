# Page states

A page that loads shows content, a loading state, an empty message, or an error. The user can press Retry, and a stale answer never lands on a page they left.

## Sub-features

- `state-healthy` shows content and no state message.
- `state-empty` says what is missing (`Nothing to show yet`, `Your library is empty`).
- `state-failed` shows the server error and `Retry`.
- `state-offline` says why the network failed and offers `Retry`.
- `state-recover` clears the message after Retry succeeds.
- `state-stale` keeps an error from one page off another.

## How to get to it (user POV)

- Open any page that loads with no network, or when YouTube returns an error or an empty body.
- Click `Retry`.

## Driving it with the headless tester

Preconditions:

- Use `s := &switchable{}` and `a := stateApp(t, s, signedIn)` from `states_test.go`. Set `s.mode` to `"offline"`, `"server"` or `"empty"`, and back to `""` to recover.
- `pageCases` in `states_test.go` lists each page with its healthy and empty text. Add a new page there.

- **Fail.** `s.mode = "server"`, `tt := open(a, "/home")`, `p.step("failed")`. The error and `Retry` show.
- **Recover.** `s.mode = ""`, `p.click("Retry", "recovered")`. `Quick picks` shows and the error text is gone.
- **Empty.** `s.mode = "empty"`. The route's `empty` message shows.

## Gotchas

- A new route needs a `pageCases` row, or its states are unproven.
- Assert absence as well as presence. The recovered frame must not contain the old error text.
