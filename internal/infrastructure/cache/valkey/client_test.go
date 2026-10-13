package valkey_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kadekutama/go-template/internal/infrastructure/cache/valkey"
)

func validTestParams() valkey.Params {
	return valkey.Params{
		Addr:         "127.0.0.1:6379",
		PoolSize:     10,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	}
}

func TestNewValkeyClient(t *testing.T) {
	t.Parallel()

	baseParams := validTestParams()

	type testCase struct {
		name           string
		params         valkey.Params
		expectedResult bool
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "valid params accepted",
			params:         baseParams,
			expectedResult: true,
			expectedError:  nil,
		},
		{
			name: "blank addr rejected",
			params: func() valkey.Params {
				p := baseParams
				p.Addr = ""

				return p
			}(),
			expectedResult: false,
			expectedError:  valkey.ErrValkeyInvalidParams1ViolationSAddrRuleRequiredOnValue,
		},
		{
			name: "whitespace addr rejected",
			params: func() valkey.Params {
				p := baseParams
				p.Addr = "   "

				return p
			}(),
			expectedResult: false,
			expectedError:  valkey.ErrValkeyAddrIsRequired,
		},
		{
			name: "invalid negative db rejected",
			params: func() valkey.Params {
				p := baseParams
				p.DB = -1

				return p
			}(),
			expectedResult: false,
			expectedError:  valkey.ErrValkeyInvalidParams1ViolationSDBRuleGteOnValue1,
		},
		{
			name: "invalid high db rejected",
			params: func() valkey.Params {
				p := baseParams
				p.DB = 16

				return p
			}(),
			expectedResult: false,
			expectedError:  valkey.ErrValkeyInvalidParams1ViolationSDBRuleLteOnValue16,
		},
		{
			name: "with tls enabled",
			params: func() valkey.Params {
				p := baseParams
				p.UseTLS = true

				return p
			}(),
			expectedResult: true,
			expectedError:  nil,
		},
		{
			name: "missing pool size rejected",
			params: func() valkey.Params {
				p := baseParams
				p.PoolSize = 0

				return p
			}(),
			expectedResult: false,
			expectedError:  valkey.ErrValkeyInvalidParams1ViolationSPoolSizeRuleRequiredOnValue,
		},
		{
			name: "negative pool size rejected",
			params: func() valkey.Params {
				p := baseParams
				p.PoolSize = -1

				return p
			}(),
			expectedResult: false,
			expectedError:  valkey.ErrValkeyInvalidParams1ViolationSPoolSizeRuleGtOnValue1,
		},
		{
			name: "missing dial timeout rejected",
			params: func() valkey.Params {
				p := baseParams
				p.DialTimeout = 0

				return p
			}(),
			expectedResult: false,
			expectedError:  valkey.ErrValkeyInvalidParams1ViolationSDialTimeoutRuleRequiredOnVa,
		},
		{
			name: "negative dial timeout rejected",
			params: func() valkey.Params {
				p := baseParams
				p.DialTimeout = -time.Second

				return p
			}(),
			expectedResult: false,
			expectedError:  valkey.ErrValkeyInvalidParams1ViolationSDialTimeoutRuleGtOnValue1s,
		},
		{
			name: "missing read timeout rejected",
			params: func() valkey.Params {
				p := baseParams
				p.ReadTimeout = 0

				return p
			}(),
			expectedResult: false,
			expectedError:  valkey.ErrValkeyInvalidParams1ViolationSReadTimeoutRuleRequiredOnVa,
		},
		{
			name: "negative read timeout rejected",
			params: func() valkey.Params {
				p := baseParams
				p.ReadTimeout = -time.Second

				return p
			}(),
			expectedResult: false,
			expectedError:  valkey.ErrValkeyInvalidParams1ViolationSReadTimeoutRuleGtOnValue1s,
		},
		{
			name: "missing write timeout rejected",
			params: func() valkey.Params {
				p := baseParams
				p.WriteTimeout = 0

				return p
			}(),
			expectedResult: false,
			expectedError:  valkey.ErrValkeyInvalidParams1ViolationSWriteTimeoutRuleRequiredOnV,
		},
		{
			name: "negative write timeout rejected",
			params: func() valkey.Params {
				p := baseParams
				p.WriteTimeout = -time.Second

				return p
			}(),
			expectedResult: false,
			expectedError:  valkey.ErrValkeyInvalidParams1ViolationSWriteTimeoutRuleGtOnValue1s,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client, err := valkey.NewValkeyClient(tc.params)
			if tc.expectedError != nil {
				assert.EqualError(t, err, tc.expectedError.Error())
				assert.Nil(t, client)

				return
			}

			require.NoError(t, err)
			require.NotNil(t, client)
			assert.Equal(t, tc.expectedResult, client != nil)
			require.NoError(t, client.Close())
		})
	}
}

func TestValkeyClientPingValidation(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		c             *valkey.Client
		ctx           context.Context
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "nil receiver rejected",
			c:             nil,
			ctx:           context.Background(),
			expectedError: valkey.ErrValkeyClientIsNotInitialized,
		},
		{
			name:          "uninitialized internal client rejected",
			c:             &valkey.Client{},
			ctx:           context.Background(),
			expectedError: valkey.ErrValkeyClientIsNotInitialized,
		},
		{
			name: "canceled context aborts ping",
			c: func() *valkey.Client {
				client, err := valkey.NewValkeyClient(validTestParams())
				if err != nil {
					panic(err)
				}

				return client
			}(),
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()

				return ctx
			}(),
			expectedError: context.Canceled,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if tc.c != nil {
				defer func() { _ = tc.c.Close() }()
			}

			err := tc.c.Ping(tc.ctx)
			require.Error(t, err)

			if errors.Is(tc.expectedError, context.Canceled) {
				assert.ErrorIs(t, err, context.Canceled)
			} else {
				assert.EqualError(t, err, tc.expectedError.Error())
			}
		})
	}
}

func TestValkeyClientGetValidation(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		c              *valkey.Client
		ctx            context.Context
		key            string
		expectedResult []byte
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "nil receiver rejected",
			c:              nil,
			ctx:            context.Background(),
			key:            "key-1",
			expectedResult: nil,
			expectedError:  valkey.ErrValkeyClientIsNotInitialized,
		},
		{
			name:           "uninitialized internal client rejected",
			c:              &valkey.Client{},
			ctx:            context.Background(),
			key:            "key-1",
			expectedResult: nil,
			expectedError:  valkey.ErrValkeyClientIsNotInitialized,
		},
		{
			name: "empty key rejected",
			c: func() *valkey.Client {
				client, err := valkey.NewValkeyClient(validTestParams())
				if err != nil {
					panic(err)
				}

				return client
			}(),
			ctx:            context.Background(),
			key:            "",
			expectedResult: nil,
			expectedError:  valkey.ErrValkeyKeyIsRequired,
		},
		{
			name: "canceled context aborts get",
			c: func() *valkey.Client {
				client, err := valkey.NewValkeyClient(validTestParams())
				if err != nil {
					panic(err)
				}

				return client
			}(),
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()

				return ctx
			}(),
			key:            "k",
			expectedResult: nil,
			expectedError:  context.Canceled,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if tc.c != nil {
				defer func() { _ = tc.c.Close() }()
			}

			actualResult, err := tc.c.Get(tc.ctx, tc.key)
			if tc.expectedError != nil {
				require.Error(t, err)

				if errors.Is(tc.expectedError, context.Canceled) {
					assert.ErrorIs(t, err, context.Canceled)
				} else {
					assert.EqualError(t, err, tc.expectedError.Error())
				}

				assert.Equal(t, tc.expectedResult, actualResult)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.expectedResult, actualResult)
		})
	}
}

func TestValkeyClientSetValidation(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		c             *valkey.Client
		ctx           context.Context
		key           string
		value         []byte
		ttl           time.Duration
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "nil receiver rejected",
			c:             nil,
			ctx:           context.Background(),
			key:           "k",
			value:         []byte(`x`),
			ttl:           time.Minute,
			expectedError: valkey.ErrValkeyClientIsNotInitialized,
		},
		{
			name:          "uninitialized internal client rejected",
			c:             &valkey.Client{},
			ctx:           context.Background(),
			key:           "k",
			value:         []byte(`x`),
			ttl:           time.Minute,
			expectedError: valkey.ErrValkeyClientIsNotInitialized,
		},
		{
			name: "empty key rejected",
			c: func() *valkey.Client {
				client, err := valkey.NewValkeyClient(validTestParams())
				if err != nil {
					panic(err)
				}

				return client
			}(),
			ctx:           context.Background(),
			key:           "",
			value:         []byte(`x`),
			ttl:           time.Minute,
			expectedError: valkey.ErrValkeyKeyIsRequired,
		},
		{
			name: "zero ttl rejected",
			c: func() *valkey.Client {
				client, err := valkey.NewValkeyClient(validTestParams())
				if err != nil {
					panic(err)
				}

				return client
			}(),
			ctx:           context.Background(),
			key:           "k",
			value:         []byte(`x`),
			ttl:           0,
			expectedError: valkey.ErrValkeyTTLMustBePositive,
		},
		{
			name: "negative ttl rejected",
			c: func() *valkey.Client {
				client, err := valkey.NewValkeyClient(validTestParams())
				if err != nil {
					panic(err)
				}

				return client
			}(),
			ctx:           context.Background(),
			key:           "k",
			value:         []byte(`x`),
			ttl:           -time.Second,
			expectedError: valkey.ErrValkeyTTLMustBePositive,
		},
		{
			name: "canceled context aborts set",
			c: func() *valkey.Client {
				client, err := valkey.NewValkeyClient(validTestParams())
				if err != nil {
					panic(err)
				}

				return client
			}(),
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()

				return ctx
			}(),
			key:           "k",
			value:         []byte(`x`),
			ttl:           time.Minute,
			expectedError: context.Canceled,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if tc.c != nil {
				defer func() { _ = tc.c.Close() }()
			}

			err := tc.c.Set(tc.ctx, tc.key, tc.value, tc.ttl)
			if tc.expectedError != nil {
				if errors.Is(tc.expectedError, context.Canceled) {
					assert.ErrorIs(t, err, tc.expectedError)
				} else {
					assert.EqualError(t, err, tc.expectedError.Error())
				}

				return
			}

			assert.NoError(t, err)
		})
	}
}

func TestValkeyClientDeleteValidation(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		c             *valkey.Client
		ctx           context.Context
		key           string
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "nil receiver rejected",
			c:             nil,
			ctx:           context.Background(),
			key:           "k",
			expectedError: valkey.ErrValkeyClientIsNotInitialized,
		},
		{
			name:          "uninitialized internal client rejected",
			c:             &valkey.Client{},
			ctx:           context.Background(),
			key:           "k",
			expectedError: valkey.ErrValkeyClientIsNotInitialized,
		},
		{
			name: "empty key rejected",
			c: func() *valkey.Client {
				client, err := valkey.NewValkeyClient(validTestParams())
				if err != nil {
					panic(err)
				}

				return client
			}(),
			ctx:           context.Background(),
			key:           "",
			expectedError: valkey.ErrValkeyKeyIsRequired,
		},
		{
			name: "canceled context aborts delete",
			c: func() *valkey.Client {
				client, err := valkey.NewValkeyClient(validTestParams())
				if err != nil {
					panic(err)
				}

				return client
			}(),
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()

				return ctx
			}(),
			key:           "k",
			expectedError: context.Canceled,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if tc.c != nil {
				defer func() { _ = tc.c.Close() }()
			}

			err := tc.c.Delete(tc.ctx, tc.key)
			if tc.expectedError != nil {
				if errors.Is(tc.expectedError, context.Canceled) {
					assert.ErrorIs(t, err, tc.expectedError)
				} else {
					assert.EqualError(t, err, tc.expectedError.Error())
				}

				return
			}

			assert.NoError(t, err)
		})
	}
}

func TestValkeyClientTTLValidation(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		c              *valkey.Client
		ctx            context.Context
		key            string
		expectedResult time.Duration
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "nil receiver rejected",
			c:              nil,
			ctx:            context.Background(),
			key:            "k",
			expectedResult: 0,
			expectedError:  valkey.ErrValkeyClientIsNotInitialized,
		},
		{
			name:           "uninitialized internal client rejected",
			c:              &valkey.Client{},
			ctx:            context.Background(),
			key:            "k",
			expectedResult: 0,
			expectedError:  valkey.ErrValkeyClientIsNotInitialized,
		},
		{
			name: "empty key rejected",
			c: func() *valkey.Client {
				client, err := valkey.NewValkeyClient(validTestParams())
				if err != nil {
					panic(err)
				}

				return client
			}(),
			ctx:            context.Background(),
			key:            "",
			expectedResult: 0,
			expectedError:  valkey.ErrValkeyKeyIsRequired,
		},
		{
			name: "canceled context aborts ttl",
			c: func() *valkey.Client {
				client, err := valkey.NewValkeyClient(validTestParams())
				if err != nil {
					panic(err)
				}

				return client
			}(),
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()

				return ctx
			}(),
			key:            "k",
			expectedResult: 0,
			expectedError:  context.Canceled,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if tc.c != nil {
				defer func() { _ = tc.c.Close() }()
			}

			actualResult, err := tc.c.TTL(tc.ctx, tc.key)
			if tc.expectedError != nil {
				require.Error(t, err)

				if errors.Is(tc.expectedError, context.Canceled) {
					assert.ErrorIs(t, err, context.Canceled)
				} else {
					assert.EqualError(t, err, tc.expectedError.Error())
				}

				assert.Equal(t, tc.expectedResult, actualResult)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.expectedResult, actualResult)
		})
	}
}

func TestValkeyClientSetNXValidation(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		c              *valkey.Client
		ctx            context.Context
		key            string
		value          []byte
		ttl            time.Duration
		expectedResult bool
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "nil receiver rejected",
			c:              nil,
			ctx:            context.Background(),
			key:            "k",
			value:          []byte(`x`),
			ttl:            time.Minute,
			expectedResult: false,
			expectedError:  valkey.ErrValkeyClientIsNotInitialized,
		},
		{
			name:           "uninitialized internal client rejected",
			c:              &valkey.Client{},
			ctx:            context.Background(),
			key:            "k",
			value:          []byte(`x`),
			ttl:            time.Minute,
			expectedResult: false,
			expectedError:  valkey.ErrValkeyClientIsNotInitialized,
		},
		{
			name: "empty key rejected",
			c: func() *valkey.Client {
				client, err := valkey.NewValkeyClient(validTestParams())
				if err != nil {
					panic(err)
				}

				return client
			}(),
			ctx:            context.Background(),
			key:            "",
			value:          []byte(`x`),
			ttl:            time.Minute,
			expectedResult: false,
			expectedError:  valkey.ErrValkeyKeyIsRequired,
		},
		{
			name: "zero ttl rejected",
			c: func() *valkey.Client {
				client, err := valkey.NewValkeyClient(validTestParams())
				if err != nil {
					panic(err)
				}

				return client
			}(),
			ctx:            context.Background(),
			key:            "k",
			value:          []byte(`x`),
			ttl:            0,
			expectedResult: false,
			expectedError:  valkey.ErrValkeyTTLMustBePositive,
		},
		{
			name: "negative ttl rejected",
			c: func() *valkey.Client {
				client, err := valkey.NewValkeyClient(validTestParams())
				if err != nil {
					panic(err)
				}

				return client
			}(),
			ctx:            context.Background(),
			key:            "k",
			value:          []byte(`x`),
			ttl:            -time.Second,
			expectedResult: false,
			expectedError:  valkey.ErrValkeyTTLMustBePositive,
		},
		{
			name: "canceled context aborts setnx",
			c: func() *valkey.Client {
				client, err := valkey.NewValkeyClient(validTestParams())
				if err != nil {
					panic(err)
				}

				return client
			}(),
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()

				return ctx
			}(),
			key:            "k",
			value:          []byte(`x`),
			ttl:            time.Minute,
			expectedResult: false,
			expectedError:  context.Canceled,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if tc.c != nil {
				defer func() { _ = tc.c.Close() }()
			}

			actualResult, err := tc.c.SetNX(tc.ctx, tc.key, tc.value, tc.ttl)
			if tc.expectedError != nil {
				require.Error(t, err)

				if errors.Is(tc.expectedError, context.Canceled) {
					assert.ErrorIs(t, err, context.Canceled)
				} else {
					assert.EqualError(t, err, tc.expectedError.Error())
				}

				assert.Equal(t, tc.expectedResult, actualResult)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.expectedResult, actualResult)
		})
	}
}

func TestValkeyClientEvalValidation(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		c              *valkey.Client
		ctx            context.Context
		script         string
		keys           []string
		args           []any
		expectedResult any
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "nil receiver rejected",
			c:              nil,
			ctx:            context.Background(),
			script:         "return 1",
			keys:           []string{"k"},
			args:           nil,
			expectedResult: nil,
			expectedError:  valkey.ErrValkeyClientIsNotInitialized,
		},
		{
			name:           "uninitialized internal client rejected",
			c:              &valkey.Client{},
			ctx:            context.Background(),
			script:         "return 1",
			keys:           []string{"k"},
			args:           nil,
			expectedResult: nil,
			expectedError:  valkey.ErrValkeyClientIsNotInitialized,
		},
		{
			name: "canceled context aborts eval",
			c: func() *valkey.Client {
				client, err := valkey.NewValkeyClient(validTestParams())
				if err != nil {
					panic(err)
				}

				return client
			}(),
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()

				return ctx
			}(),
			script:         "return 1",
			keys:           []string{"k"},
			args:           nil,
			expectedResult: nil,
			expectedError:  context.Canceled,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if tc.c != nil {
				defer func() { _ = tc.c.Close() }()
			}

			actualResult, err := tc.c.Eval(tc.ctx, tc.script, tc.keys, tc.args...)
			if tc.expectedError != nil {
				require.Error(t, err)

				if errors.Is(tc.expectedError, context.Canceled) {
					assert.ErrorIs(t, err, context.Canceled)
				} else {
					assert.EqualError(t, err, tc.expectedError.Error())
				}

				assert.Equal(t, tc.expectedResult, actualResult)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.expectedResult, actualResult)
		})
	}
}

func TestValkeyClientClose(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		c             *valkey.Client
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "nil receiver succeeds",
			c:             nil,
			expectedError: nil,
		},
		{
			name:          "uninitialized internal client succeeds",
			c:             &valkey.Client{},
			expectedError: nil,
		},
		{
			name: "initialized client closes cleanly",
			c: func() *valkey.Client {
				client, err := valkey.NewValkeyClient(validTestParams())
				if err != nil {
					panic(err)
				}

				return client
			}(),
			expectedError: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := tc.c.Close()
			if tc.expectedError != nil {
				assert.EqualError(t, err, tc.expectedError.Error())

				return
			}

			assert.NoError(t, err)
		})
	}
}
