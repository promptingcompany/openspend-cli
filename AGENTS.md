# AGENTS.md

## Completion Checklist (Required)

Before declaring any change in this repository done, run:

1. `make cli-test-loop`
2. `make cli-test-local-openspend`

Use `make cli-test-real-backend` instead of local when validating against a non-local backend target.

If integration tests cannot run because the backend is unavailable, explicitly report that blocker and do not claim full completion.
