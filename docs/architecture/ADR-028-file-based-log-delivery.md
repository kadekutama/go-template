# ADR-028: File-based log delivery with rotation for server environments

**Status:** Accepted
**Date:** 2026-10-22
**Approved By:** repository owner (directed 2026-10-22)
**Owning task:** `E09.3-T01`

## Context

The repository already emits structured zerolog JSON through the kernel
`log.Logger` port (ADR-010 port, ADR-012 backend). The binary bootstrap writes
all records to `os.Stdout`, which is convenient for local development but does
not provide durable, rotated files for a server-side log processor. Test
fixtures also construct loggers with `io.Discard`, making diagnostics
invisible.

The owner directed that local runs keep logs visible in the terminal, while
server environments write rotating log files that are then shipped to a log
processor. The repository does not define a single processor product or
protocol, and does not yet own the full E15 observability collector pipeline.

## Decision

1. The logging adapter supports `stdout`, `file`, and `both` output modes.
2. Environment resolution defaults to `stdout` for local runs and `file` for
   explicit `staging`/`production` environments. `APP_LOG_OUTPUT` (or
   `APP_OBSERVABILITY__LOG_OUTPUT`) overrides the default.
3. File mode writes newline-delimited JSON through `lumberjack` with
   configurable maximum size, backup count, maximum age, and compression. A
   file path is mandatory; an invalid or missing path fails startup.
4. The application owns durable file production only. A log-processor sidecar
   (for example Vector) tails the JSON file and pushes it to the processor's
   configured endpoint. Provider-specific network delivery is not embedded in
   every binary; E15-T01 remains the owner of the full collector/Loki pipeline.
5. The binary entrypoint's logger is reused by the fx graph so a single
   rotation writer owns the file per process.
6. `io.Discard` is not used as a logger sink. It remains acceptable solely for
   intentionally draining bounded HTTP response bodies.

## Consequences

- Local development keeps immediate terminal feedback.
- Staging/production can persist logs independently of stdout capture and can
  hand a stable JSON-lines file to an external log processor.
- Operators must provision a writable log directory and a sidecar/agent, and
  must set `APP_LOG_FILE` for file mode.
- File rotation is local to each process; rotating across multiple replicas
  requires per-pod files or an external collector, not a shared writable file.
- The kernel logging port and domain/application dependency direction remain
  unchanged.

## Alternatives considered

- **Keep stdout only:** rejected; does not satisfy durable rotating server files
  or the explicit log-processor handoff.
- **Embed a vendor-specific shipper in every binary:** rejected; the repository
  has no selected processor protocol, and network delivery from the logger
  would add retries, backpressure, credentials, and failure semantics outside
  the logging port.
- **Use a shared writable file across replicas:** rejected; rotation and
  concurrent writers would corrupt or misorder records. Per-process files or a
  sidecar are required.
