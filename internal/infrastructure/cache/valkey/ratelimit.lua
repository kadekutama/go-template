-- Token-bucket rate limiter (E08-T07, SPEC 9.4). The ONLY redis.call Lua in the repo.
-- State per key is a hash {tokens (float), ts (ms)} refilled continuously at
-- budget/window tokens per ms, so bursts up to budget pass and sustained
-- traffic is admitted at the configured rate.
-- KEYS[1] = key ("ratelimit:{dimension}:{id}")
-- ARGV[1] = budget (int > 0, max tokens + burst)
-- ARGV[2] = window_ms (int > 0, refill horizon for a full bucket)
-- ARGV[3] = now_ms (int, caller clock for determinism)
-- Returns {allowed (1/0), remaining (int, floor), retry_after_ms (int)}.
local key = KEYS[1]
local budget = tonumber(ARGV[1])
local window_ms = tonumber(ARGV[2])
local now_ms = tonumber(ARGV[3])

if not budget or budget <= 0 then
  return {0, 0, 0}
end
if not window_ms or window_ms <= 0 then
  return {0, 0, 0}
end
if not now_ms or now_ms < 0 then
  return {0, 0, 0}
end

local rate = budget / window_ms

local stored = redis.call('HMGET', key, 'tokens', 'ts')
local tokens = tonumber(stored[1])
local ts = tonumber(stored[2])
if not tokens then
  tokens = budget
end
if not ts then
  ts = now_ms
end

local elapsed = now_ms - ts
if elapsed < 0 then
  elapsed = 0
end
tokens = math.min(budget, tokens + elapsed * rate)

if tokens >= 1 then
  tokens = tokens - 1
  redis.call('HSET', key, 'tokens', tokens, 'ts', now_ms)
  redis.call('PEXPIRE', key, window_ms * 2)
  return {1, math.floor(tokens), 0}
end

local retry_ms = math.ceil((1 - tokens) / rate)
redis.call('HSET', key, 'tokens', tokens, 'ts', now_ms)
local ttl = redis.call('PTTL', key)
if ttl < 0 then
  ttl = retry_ms
  redis.call('PEXPIRE', key, retry_ms + window_ms)
end
return {0, 0, retry_ms}
