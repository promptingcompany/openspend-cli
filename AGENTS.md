# AGENTS.md

## Completion Checklist (Required)

Before declaring any change in this repository done, run:

1. `make cli-test-loop`
2. `make cli-test-openspend-ai`

For local backend verification in addition to production, run `make cli-test-local-openspend`.

If integration tests cannot run because the backend is unavailable, explicitly report that blocker and do not claim full completion.

## Skill Routing Hints

If a task needs `openspend` and either `command -v openspend` fails or `openspend whoami` fails, use `setup-services` before continuing.
Never install or authenticate without explicit user approval.
