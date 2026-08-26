# Omasub Build Plan

## Product Definition

Omasub is an identical, rebranded clone of the reference Subsonic Bubble Tea
client. It keeps the complete feature set, workflows, storage behavior, audio
stack, keyboard controls, and terminal UI behavior.

The only intentional application-level changes are:

1. Replace all legacy branding with Omasub and Omarchy-native presentation.
2. Use the current Omarchy default agent as the default curator provider while
   retaining Ollama and vLLM as selectable alternatives.

The companion Omarchy widget is not another music client. It is a small remote
control and status display for a running Omasub process.

## Non-Negotiable Constraints

- Preserve behavior unless a change is explicitly listed in this plan.
- Keep Bubble Tea, Bubbles, Lip Gloss, mpv, SQLite, the encrypted vault,
  Subsonic/Navidrome support, and the existing curator safeguards.
- First-party source files must not exceed 300 lines. Split by responsibility,
  not by arbitrary line ranges.
- Application packages must not contain legacy brand names, identifiers, paths,
  environment variables, UI copy, prompts, test fixtures, or API client names.
- The Omasub repository remains independent. It does not import code from the
  reference checkout at build time.
- Do not initialize or use Git until a mostly-functional binary exists and the
  user explicitly approves it.
- Never modify packaged files under `/usr/share/omarchy/`.
- The widget ships as repository source and installs as a user-owned Omarchy
  shell plugin under `~/.config/omarchy/plugins/`.

## Visual Identity

Use this banner verbatim:

```text
________                   _________    ________
\_____  \   _____ _____   /   _____/__ _\_____  \
 /   |   \ /     \\__  \  \_____  \|  |  \_(__  <
/    |    \  Y Y  \/ __ \_/        \  |  /       \
\_______  /__|_|  (____  /_______  /____/______  /
        \/      \/     \/        \/            \/
```

The TUI palette must resolve from the active Omarchy theme. Semantic colors
should cover primary text, accent, muted text, success, warning, error,
selection, and borders. Terminal-safe fallbacks are allowed when Omarchy theme
data is unavailable. The widget must consume Omarchy shell theme properties
directly. Both surfaces must remain readable under dark and light themes.

## Phase 1: Clean Clone and Mechanical Rename

- Copy the reference application's source, tests, documentation, license, and
  required non-generated assets into this repository.
- Exclude its Git metadata, module/build caches, compiled binaries, screenshots,
  and other generated output.
- Rename the Go module, command directory, binary, installers, imports, config
  paths, data paths, environment variables, process names, socket names, and
  Subsonic API client identifier to `omasub`/`OMASUB_*` equivalents.
- Replace user-visible branding while preserving layout and behavior.
- Add a case-insensitive forbidden-brand audit for first-party source, tests,
  prompts, filenames, and shipped documentation.
- Establish a baseline build and test report before behavioral work.

Exit criteria:

- The cloned application builds or has only explicitly documented mechanical
  rename failures.
- No forbidden legacy identifier remains in shipped application material.
- No source or state is read from the old application's directories.

## Phase 2: Structural Refactor Without Behavior Changes

Bring every oversized first-party source file below 300 lines while keeping the
application behavior identical.

Primary splits:

- Curator orchestration, prompt construction, streaming, parsing, validation,
  repair, and run metadata.
- TUI curator preparation, execution, progress messages, and result application.
- Bubble Tea update routing by application mode and message domain.
- Subsonic transport, authentication, browsing, media, and playlist operations.
- TUI model state grouped into focused domain structures.
- Queue selection, mutation, navigation, and view helpers.
- Grounding interpretation, candidate search, and anchor resolution.
- Large tests divided by behavior under test.

Add a line-count check that fails when a first-party source file exceeds 300
lines.

Exit criteria:

- Every first-party source file is at most 300 lines.
- The full inherited behavior test suite passes.
- No feature or keybinding has been intentionally changed.

## Phase 3: Omarchy-Native Branding and Theme Adapter

- Install the supplied banner in the existing banner/header location without
  changing the surrounding TUI structure.
- Replace remaining UI copy, curator identity, fallback playlist names, spinner
  copy, examples, prompts, fixtures, and documentation with neutral Omasub or
  Omarchy language.
- Add a small theme package that reads the active Omarchy color contract and
  maps it to Lip Gloss semantic roles.
- Keep theme discovery isolated so the rest of the TUI depends only on semantic
  color names.
- Provide a deterministic fallback palette for non-Omarchy systems and tests.
- Add dark/light contrast-oriented rendering tests where practical.

Exit criteria:

- The supplied banner renders correctly.
- Changing the active Omarchy theme updates a running Omasub session promptly.
- The TUI remains usable when Omarchy theme data is unavailable.
- The forbidden-brand audit passes.

## Phase 4: Curator Provider Abstraction

Retain the existing closed-world curator contract: providers receive only the
bounded candidate data required for curation, and returned track IDs are
validated against the encrypted local cache before use.

Support three providers behind one completion interface:

1. `omarchy` — default for new configurations.
2. `ollama` — existing local HTTP integration.
3. `vllm` — existing OpenAI-compatible HTTP integration.

For the Omarchy provider:

- Resolve the selected executable through `omarchy default agent` at request
  time so changing the system default does not require reconfiguring Omasub.
- Return an actionable error when no default is selected or the selected agent
  is unavailable.
- Execute in an isolated temporary working directory with a strict structured-
  output prompt and no vault/config paths in the working context.
- Implement agent-specific non-interactive runners behind a common interface;
  begin with the currently selected agent and cover the agents supported by the
  installed Omarchy dispatcher.
- Capture and normalize output before passing it to the existing parser and
  validator.
- Preserve cancellation, timeouts, repair rounds, progressive application where
  supported, and encrypted recommendation-run metadata.
- Record `omarchy` plus the resolved agent name as provider metadata.

For configuration:

- Fresh installs select `omarchy` automatically.
- The in-app provider selector lists Omarchy first, then Ollama and vLLM.
- Omarchy requires no URL or model-selection screens.
- Ollama and vLLM retain endpoint normalization, model discovery, manual model
  entry, context settings, API key support, and environment overrides.

Exit criteria:

- A fresh Omasub config is curator-ready when an Omarchy default agent exists.
- Curator workflows operate through the selected Omarchy agent.
- Ollama and vLLM remain functional alternatives.
- Invalid, noisy, or malicious provider output cannot insert unknown track IDs.

## Phase 5: Local Remote-Control Bridge

Add the smallest interface needed by the widget without altering the TUI's
music behavior.

- Keep the running TUI/player process as the single owner of playback state.
- Publish an atomic, minimal now-playing snapshot under the appropriate XDG
  runtime/state location.
- Include only safe display/control fields: running state, playback state,
  title, artist, album, duration/progress when available, and update time.
- Never expose server credentials, vault data, playlists, candidate crates, or
  curator prompts.
- Add a local control channel for play/pause, next, previous, and stop.
- Add a launch/focus action for cases where Omasub is not running.
- Keep playback, queue state, and the unlocked vault in one persistent listening
  session when its terminal UI is detached.
- Add an explicit quit command that ends playback, closes and relocks the vault,
  and terminates the listening session.
- Detect stale state and stale sockets deterministically.
- Keep transport and protocol code separate from player and TUI logic.

Exit criteria:

- A headless status command accurately describes the running player.
- Remote commands affect the existing Omasub process rather than starting a
  competing player.
- Stale or absent processes fail safely and clearly.

## Phase 6: Omarchy Remote/Status Widget

Create a native Quickshell plugin in the repository with an Omarchy-compliant
manifest and files below 300 lines.

Bar surface:

- Compact Omasub/music identity.
- Playing/paused/stopped indication.
- Concise track identity when space permits.
- Clear inactive state when Omasub is not running.

Panel surface:

- Track title, artist, and album.
- Playback status and progress when available.
- Previous, play/pause, next, and stop controls.
- One label-changing playback-context control cycling normal, shuffle,
  shuffle/repeat, repeat all, and repeat one.
- Launch/focus Omasub action.
- Explicit quit-and-lock action, separate from stopping playback.
- No library browsing, setup, queue editing, playlist management, or curator UI.

Integration requirements:

- Read the bridge snapshot and invoke only the bridge's explicit commands.
- Route hardware media keys to the live Omasub bridge, including Alt+Play and
  Alt+Shift+Play fallbacks for keyboards without dedicated track keys; retain
  Omarchy's selected media source as the fallback when Omasub is inactive.
- Use Omarchy shell theme properties rather than hardcoded colors.
- Hot-reload cleanly as a user-owned plugin.
- Handle missing binaries, stopped playback, stale state, and command failures
  without breaking the shell.
- Provide install/uninstall helpers that touch only user-owned paths.

Exit criteria:

- The widget reflects a running Omasub session promptly.
- All remote controls operate on that session.
- The widget launches Omasub when inactive.
- Clicking an active widget focuses its terminal; closing that terminal detaches
  the UI without ending playback, losing the queue, or relocking the vault.
- Quit from the widget ends the persistent session and clears the vault key.
- Theme changes are reflected through Omarchy's normal shell theming.

## Phase 7: Mostly-Functional Binary Gate

Before any Git discussion:

- Run the complete test suite and `go vet`.
- Build a clean local `omasub` binary.
- Exercise first-run setup using disposable config and data homes.
- Verify config, vault, and database permissions.
- Smoke-test server connection, browsing, search, cache sync, queue management,
  playback, history, private playlists, server playlist operations, persistence,
  cover art, and visualizer behavior.
- Exercise the Omarchy provider and its malformed-output paths.
- Exercise Ollama and vLLM configuration and request paths with available or
  mocked endpoints.
- Test remote status and every control command.
- Validate the widget manifest and perform a live user-plugin smoke test.
- Re-run the forbidden-brand and 300-line checks.
- Build through the installer without automatically launching the application.

The gate passes when the binary is mostly functional, the widget can control
it, all automated checks pass, and remaining defects are documented and minor.
Only then may Git initialization be proposed, never assumed.

## Implementation Order

Execute phases in order. Fidelity takes priority over cleanup aesthetics:
mechanical clone, behavior-preserving modularization, branding/theme work,
provider addition, bridge, widget, then the full binary gate. Each phase must
leave the application buildable and testable before the next begins.
