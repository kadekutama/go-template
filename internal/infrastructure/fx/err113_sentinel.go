package fx

// Static failure messages. Returned errors never carry a request URL, query
// string, response body, provider error text, or credential; these constants
// keep the text stable and err113-clean. The literal [REDACTED] marker makes
// the redaction explicit in operator-facing errors.
const (
	msgRequestFailed       = "fx: provider request failed (access_key=[REDACTED])"
	msgResponseReadFailed  = "fx: provider response could not be read"
	msgProviderUnavailable = "fx: provider is unavailable"
	msgProviderRejected    = "fx: provider rejected the request"
	msgBreakerOpen         = "fx: provider circuit breaker is open"
	msgFetchTimedOut       = "fx: provider fetch timed out"
	msgFetchFailed         = "fx: rate fetch failed"
)
