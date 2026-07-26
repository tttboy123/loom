# S3-W4 Verification Repair 2 RED

- Date: `2026-07-26`
- Trigger: second exact required 30-run race matrix

## Command

```text
go test -race ./internal/supervisor ./internal/runtime/piadapter -count=30
```

## Result

`RED` — Supervisor passed 30 race runs. Pi adapter failed after 466.393s:

```text
TestPiExecutionAdapterConfigEnvironmentAndBinding:
adapter aliased config: ... Pi execution process failed
read |0: file already closed
```

## Root cause

The adapter called `StdoutPipe` and `StderrPipe`, started `Cmd.Wait`
immediately in a goroutine, and read both pipes concurrently. The installed Go
standard-library contract states that `Cmd.Wait` closes those pipes after
process exit and that calling `Wait` before all reads complete is incorrect.
Fast successful fixture exit therefore allowed `Wait` to close a pipe while a
reader was still draining it.

The minimal repair remains inside the frozen adapter file: use caller-owned
`os.Pipe` read ends assigned to `Cmd.Stdout` and `Cmd.Stderr`, close the
parent's writer copies immediately after `Start`, and retain explicit bounded
reader/process cleanup. No public API, protocol, environment, authority, or
capability changes.

VERDICT: RED
