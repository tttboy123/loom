# S3-W4 Verification Repair 2 Independent Problem Analysis

- Date: `2026-07-26`
- Analyst: `/root/s2_w38_problem_analysis`
- Mode: independent read-only

## Conclusion

The failure is a product-level pipe/`Wait` coordination defect, not a fixture
defect.

The adapter obtained `StdoutPipe` and `StderrPipe`, then started
`command.Wait()` concurrently before the readers had drained those pipes. The
Go standard-library contract says `Wait` closes those managed pipes after
process exit and must not be called before all reads complete. A legal
fast-exiting helper could therefore make `Wait` close stdout or stderr before
the reader observed EOF, producing the exact `file already closed` failure.

The Analyst confirmed the minimal robust repair:

- use caller-owned `os.Pipe` stdout/stderr readers;
- assign the writer ends to `Cmd.Stdout` and `Cmd.Stderr`;
- close the parent's writer copies immediately after successful `Start`;
- close readers on all return paths;
- retain immediate `Wait`, bounded process cleanup, and joined cleanup errors.

Treating `os.ErrClosed` as EOF or delaying the fixture would be unsafe because
it could accept truncated protocol or diagnostic bytes. The repair adds no API,
dependency, authority, Provider, process-group, or capability boundary.

VERDICT: CONFIRMED
