# ADR-024: Provider Egress Rate Limits over the Shared Token Bucket

**Status:** Proposed
**Date:** 2026-10-04
**Owner:** E09.1-T03
**Related tasks:** `tasks/specs/E09.1-T03.md`

## Context

Outbound provider calls (flags, FX, payment processors, SMTP) run from every
replica. Without a shared quota, retries and bursts multiply a vendor quota
by replica count. E08-T07 already provides an atomic Valkey token bucket
through `port.RateLimiter` with an accepted caller-clock contract; a second
Lua script or a guard amendment would fork those semantics.

## Decision

Add a blocking `EgressLimiter.Wait(ctx, provider, operation)` over the
existing `port.RateLimiter` (no new Lua, no guard change):

1. **Provider-operation keying.** Keys use E05-T03
   `BuildRateLimitKey(provider, operation)` → `ratelimit:{provider}:{operation}`,
   carrying no tenant PII. Quotas are explicit per-operation
   budget/window/max-wait triples; missing or invalid policy fails closed.
2. **Fail-closed outages.** Limiter denials, wait timeouts, and store errors
   are terminal for the current provider operation. They are not retried as
   provider errors and never count as provider-breaker failures. Store errors
   fail closed for outbound requests; callers may serve a valid/stale cached
   read instead of calling out.
3. **Retry placement.** E10 attempt composition per call:
   retry loop → egress `Wait` → breaker-wrapped single provider attempt →
   `httpclient.Doer`. Each retry reacquires a permit. Positive-only jitter
   applies after `RetryAfter`; waits never exceed max-wait; pre-canceled
   contexts perform no port call and mid-wait cancellation stops before
   another `Allow`. A permit reserved atomically before a racing cancellation
   is consumed, never refunded.
4. **Clock contract.** The existing single Lua script and caller-clock
   behavior are intentionally unchanged; deployment hosts must keep
   synchronized clocks (documented, not re-litigated here).

## Consequences

- Bursts are bounded by the token-bucket budget across all replicas
   (proven by the Testcontainers burst test: exactly initial capacity
   admitted with bounded max-wait shorter than one refill interval).
- E10-T01/T02/T03/T05 own the provider call-site wiring against this
   contract; ingress 429 mapping stays with E08-T07/E11.

## Rollback

Remove the egress package and its policies; provider call sites return to
direct breaker-guarded calls. No data migration is involved.
