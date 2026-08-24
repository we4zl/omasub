#!/usr/bin/env bash

set -euo pipefail

# Omarchy shell processes may intentionally set NO_COLOR for command output.
# Omasub is a themed TUI, so do not pass that flag into its listening session.
unset NO_COLOR

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

attached=$(tmux display-message -p -t omasub '#{session_attached}' 2>/dev/null || printf '0')
if [[ $attached != 0 ]] && omasub remote status 2>/dev/null | jq -e '.running == true' >/dev/null; then
  if focus_omasub; then
    exit 0
  fi
fi

if tmux has-session -t omasub 2>/dev/null; then
  terminal_args=(tmux attach-session -t omasub)
else
  terminal_args=(tmux new-session -s omasub env -u NO_COLOR omasub)
fi

omarchy-launch-tui --app-id=org.omarchy.omasub "${terminal_args[@]}" >/dev/null 2>&1 &

for _ in {1..30}; do
  attached=$(tmux display-message -p -t omasub '#{session_attached}' 2>/dev/null || printf '0')
  if [[ $attached != 0 ]]; then
    if focus_omasub; then
      exit 0
    fi
  fi
  sleep 0.1
done

exit 1
