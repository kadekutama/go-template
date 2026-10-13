# Provider Egress Policies (E09.1-T03 format)

Per-provider outbound quotas for the distributed egress limiter. Each file
lists explicit quotas; **there is no default or unlimited quota** — a
provider operation without a policy fails closed.

## File shape (YAML)

```yaml
policies:
  - provider: unleash
    operation: flags
    budget: 100      # token-bucket admissions per window (positive integer)
    window: 1m       # Go duration string, positive (e.g. 500ms, 1m)
    max_wait: 5s     # max one Wait call blocks, positive
```

## Rules

- `provider` / `operation` are colon-safe key components: 1–128 bytes, no
  `:`, no surrounding whitespace, no control characters. Keys become
  `ratelimit:{provider}:{operation}` through the E05 key builder.
- `budget`, `window`, and `max_wait` must all be positive; zero or missing
  values are rejected at load with a typed error.
- Per-provider files (`unleash.yaml`, `fx.yaml`, `payment-processor.yaml`,
  `smtp.yaml`) are owned by E10 integration tasks, which also own the
  call-site wiring: retry loop → egress `Wait` → breaker-wrapped single
  provider attempt → `httpclient.Doer`. Each retry reacquires a permit;
  limiter/store errors are terminal for that call and never count as
  provider-breaker failures.

## Semantics

- Admission comes from the shared E08-T07 Valkey token bucket; this package
  adds keying, validation, and bounded blocking waits with positive-only
  jitter after `RetryAfter`. See
  `docs/architecture/ADR-024-provider-egress-rate-limits.md`.
- Limiter-store errors fail closed for outbound calls; callers may serve a
  valid/stale cached read without making a provider request.
