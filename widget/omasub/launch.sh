#!/usr/bin/env bash

set -euo pipefail

# Omarchy shell may set NO_COLOR for command output. Omasub is a themed TUI.
unset NO_COLOR
OMASUB_BIN=${1:-${OMASUB_BIN:-"$HOME/.local/bin/omasub"}}

focus_omasub() {
  local client_pid terminal_pid address
  while read -r client_pid; do
    terminal_pid=$(ps -o ppid= -p "$client_pid" | tr -d ' ')
    [[ -n $terminal_pid ]] || continue
    address=$(hyprctl clients -j | jq -r --argjson pid "$terminal_pid" '
      .[] | select(.pid == $pid) | .address
    ' | head -n 1)
    [[ -n $address ]] || continue
    hyprctl dispatch "hl.dsp.focus({ window = \"address:$address\" })" >/dev/null
    return 0
  done < <(tmux list-clients -t omasub -F '#{client_pid}' 2>/dev/null)
  return 1
}

configure_visibility_hooks() {
  local bin_q
  printf -v bin_q '%q' "$OMASUB_BIN"
  tmux set-hook -t omasub client-attached "run-shell '$bin_q remote visible >/dev/null 2>&1'"
  tmux set-hook -t omasub client-detached "run-shell '$bin_q remote hidden >/dev/null 2>&1'"
}

if tmux has-session -t omasub 2>/dev/null; then
  configure_visibility_hooks
fi

attached=$(tmux display-message -p -t omasub '#{session_attached}' 2>/dev/null || printf '0')
if [[ $attached != 0 ]] && "$OMASUB_BIN" remote status 2>/dev/null | jq -e '.running == true' >/dev/null; then
  if focus_omasub; then
    exit 0
  fi
fi

if tmux has-session -t omasub 2>/dev/null; then
  terminal_args=(tmux attach-session -t omasub)
else
  tmux new-session -d -s omasub env -u NO_COLOR "$OMASUB_BIN"
  terminal_args=(tmux attach-session -t omasub)
fi

configure_visibility_hooks

omarchy-launch-tui --app-id=org.omarchy.omasub "${terminal_args[@]}" >/dev/null 2>&1 &

for _ in {1..30}; do
  attached=$(tmux display-message -p -t omasub '#{session_attached}' 2>/dev/null || printf '0')
  if [[ $attached != 0 ]] && focus_omasub; then
    "$OMASUB_BIN" remote visible >/dev/null 2>&1 || true
    exit 0
  fi
  sleep 0.1
done

exit 1
