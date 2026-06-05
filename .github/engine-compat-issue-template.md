# Engine Compatibility Failure

The nightly engine compatibility test suite has detected a failure.

## What failed

- **Date**: {{ date | date('Do MMM YYYY') }}
- **Workflow run**: {{ env.GITHUB_SERVER_URL }}/{{ env.GITHUB_REPOSITORY }}/actions/runs/{{ env.GITHUB_RUN_ID }}

## What this means

One or more engine capability tests failed. This usually means:

1. **A flag was renamed** — e.g. llama.cpp renamed `-fa` → `--flash-attn` between builds.
2. **A new required flag was added** — a feature now needs a flag that didn't exist in an older build.
3. **A flag was removed** — a build dropped support for a feature bloc relies on.

## How to fix

1. Check the [workflow run logs]({{ env.GITHUB_SERVER_URL }}/{{ env.GITHUB_REPOSITORY }}/actions/runs/{{ env.GITHUB_RUN_ID }}) for the exact test failure.
2. Update `BuildCapabilities()` in `cli/internal/engine/capabilities.go` if a flag was renamed.
3. Update `BuildArgs()` in the affected engine package if the flag syntax changed.
4. Add a new entry to the compatibility matrix in `cli/internal/engine/llamacpp/engine_test.go`.

## Affected engine versions

See the workflow matrix in `.github/workflows/engine-compat.yml`.
