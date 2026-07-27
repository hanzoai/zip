# DEPRECATED

This repository is dead. It is unmaintained and must not be imported.

**Canonical: [github.com/zap-proto/zip](https://github.com/zap-proto/zip)**

`github.com/hanzoai/zip` was a fork. Development continued on
`github.com/zap-proto/zip`, which is the only maintained line and is far ahead
of anything here.

## Migrating

Change the module path — `github.com/hanzoai/zip` → `github.com/zap-proto/zip`
— in `go.mod` and every import.

The canonical framework builds on `github.com/zap-proto/fiber` and
`github.com/zap-proto/http`, not upstream `github.com/gofiber/fiber`, so expect
API drift. It is not a drop-in swap.

## Branches kept for reference

Nothing is deleted; the old lines are preserved as branches.

| Branch | Holds |
| --- | --- |
| `backup/hanzoai-zip-final` | Final state of the old `master` line, which was never merged into `main`. |
| `backup/hanzoai-zip-final-stash` | A work-in-progress `module.go` refactor recovered from a dropped stash. |
| `backup/dbc-zip-master` | Earlier backup of the same line. |
