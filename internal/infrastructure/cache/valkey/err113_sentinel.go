package valkey

import "errors"

// Static error sentinels for err113; messages preserved.
var (
	ErrValkeyInvalidParams1ViolationSAddrRuleRequiredOnValue     = errors.New("valkey: invalid params (1 violation(s)): Addr: rule \"required\" on value : validation violation")
	ErrValkeyAddrIsRequired                                      = errors.New("valkey: addr is required")
	ErrValkeyInvalidParams1ViolationSDBRuleGteOnValue1           = errors.New("valkey: invalid params (1 violation(s)): DB: rule \"gte\" on value -1: validation violation")
	ErrValkeyInvalidParams1ViolationSDBRuleLteOnValue16          = errors.New("valkey: invalid params (1 violation(s)): DB: rule \"lte\" on value 16: validation violation")
	ErrValkeyInvalidParams1ViolationSPoolSizeRuleRequiredOnValue = errors.New("valkey: invalid params (1 violation(s)): PoolSize: rule \"required\" on value 0: validation violation")
	ErrValkeyInvalidParams1ViolationSPoolSizeRuleGtOnValue1      = errors.New("valkey: invalid params (1 violation(s)): PoolSize: rule \"gt\" on value -1: validation violation")
	ErrValkeyInvalidParams1ViolationSDialTimeoutRuleRequiredOnVa = errors.New("valkey: invalid params (1 violation(s)): DialTimeout: rule \"required\" on value 0s: validation violation")
	ErrValkeyInvalidParams1ViolationSDialTimeoutRuleGtOnValue1s  = errors.New("valkey: invalid params (1 violation(s)): DialTimeout: rule \"gt\" on value -1s: validation violation")
	ErrValkeyInvalidParams1ViolationSReadTimeoutRuleRequiredOnVa = errors.New("valkey: invalid params (1 violation(s)): ReadTimeout: rule \"required\" on value 0s: validation violation")
	ErrValkeyInvalidParams1ViolationSReadTimeoutRuleGtOnValue1s  = errors.New("valkey: invalid params (1 violation(s)): ReadTimeout: rule \"gt\" on value -1s: validation violation")
	ErrValkeyInvalidParams1ViolationSWriteTimeoutRuleRequiredOnV = errors.New("valkey: invalid params (1 violation(s)): WriteTimeout: rule \"required\" on value 0s: validation violation")
	ErrValkeyInvalidParams1ViolationSWriteTimeoutRuleGtOnValue1s = errors.New("valkey: invalid params (1 violation(s)): WriteTimeout: rule \"gt\" on value -1s: validation violation")
	ErrValkeyClientIsNotInitialized                              = errors.New("valkey: client is not initialized")
	ErrValkeyKeyIsRequired                                       = errors.New("valkey: key is required")
	ErrValkeyTTLMustBePositive                                   = errors.New("valkey: ttl must be positive")
	ErrRatelimitValkeyClientIsRequired                           = errors.New("ratelimit: valkey client is required")
	ErrValkeyDown                                                = errors.New("valkey down")
	ErrRatelimitUnexpectedScriptResultInterface                  = errors.New("ratelimit: unexpected script result []interface {}")
	ErrRatelimitNotInitialized                                   = errors.New("ratelimit: not initialized")
	ErrRatelimitKeyIsRequired                                    = errors.New("ratelimit: key is required")
	ErrRatelimitBudgetMustBePositive                             = errors.New("ratelimit: budget must be positive")
	ErrRatelimitWindowMustBePositive                             = errors.New("ratelimit: window must be positive")
	ErrRatelimitLuaScriptIsNotEmbedded                           = errors.New("ratelimit: lua script is not embedded")
	ErrRatelimitUnexpectedScriptResult                           = errors.New("ratelimit: unexpected script result")
	ErrUnexpectedNumber                                          = errors.New("unexpected number")
)
