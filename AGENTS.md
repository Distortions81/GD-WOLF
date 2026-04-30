# AGENTS.md

This repository is an unreleased prototype. Prefer clarity over compatibility when changing internal tooling or developer-facing command names.

## Command Naming

- Use explicit, descriptive command names for repo utilities.
- Do not keep legacy aliases just to preserve old names unless there is a concrete need.
- Current canonical utility commands:
  - `wolf-asset-export`: export decoded game assets and derived outputs.
  - `wolf-mapdump`: inspect/list decoded map data.

## Change Bias

- If a name is vague, rename it to something clearer instead of layering more documentation around the old name.
- Favor small direct changes over compatibility shims for prototype-only workflows.
