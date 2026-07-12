# Installed App Card Compact Controls Design

## Goal

Fix the installed-app grid regression where the automatic-update control and raw SDK status values overflow narrow cards.

## Card layout

- Keep the installed-app grid dense while reserving a `320px` minimum card width for readable names.
- Allow application names to use up to three lines and expose the full name through the native title tip.
- Keep icon, application identity, version, and status in the primary row.
- Replace the current full-width switch field with a compact automatic-update control.
- The visible control contains only the localized `自动更新` / `Auto update` label and the switch.
- Do not render the policy description as persistent card text.

## Tips

- Use the design-system switch `labelTooltip` for the automatic-update explanation.
- The tip explains whether the application follows scheduled client updates or requires manual updates.
- Preserve an accessible visible label and keyboard-discoverable tip.
- When saving disables the switch, use `disabledMessage` to explain that the setting is being saved.

## Status presentation

- Never display raw LazyCat SDK values such as `Status_Running` or `Status_Paused` directly.
- Normalize status case-insensitively and tolerate separators and common prefixes.
- Map known states to short localized labels:
  - running/active/started -> 运行中 / Running
  - stopped/inactive/exited -> 已停止 / Stopped
  - paused -> 已暂停 / Paused
  - starting/installing/updating -> 处理中 / Processing
  - failed/error -> 异常 / Error
  - unknown or empty -> 未知 / Unknown
- Show the normalized label as a compact status badge.
- Put the original non-empty SDK value in the badge title/tip for diagnostics.
- `有可用更新` remains higher priority than the runtime status.

## Responsive behavior

- The compact control must not contribute a `240px` minimum width to the card metadata column.
- The metadata column stays intrinsic and capped; long values cannot overlap adjacent cards or reduce names to an unusably narrow column.
- Existing mobile behavior remains two-column inside the card, with metadata and the compact update control aligned under the application identity.

## Verification

- Add pure-function tests for raw status normalization and localization keys.
- Build the frontend and verify the installed-app view at desktop width and a narrow viewport.
- Check Chinese and English labels, long application names, saving state, and unknown SDK values.
