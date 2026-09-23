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

func TestNewValkeyClient(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		params         valkey.ValkeyParams
		expectedResult bool
		expectedError  error
	}

	testCases := []testCase{
		{
			name: "addr required",
			params: valkey.ValkeyParams{
				Addr: "127.0.0.1:6379",
			},
			expectedResult: true,
			expectedError:  nil,
		},
		{
			name: "blank addr rejected",
			params: valkey.ValkeyParams{
				Addr: "",
			},
			expectedResult: false,
			expectedError:  errors.New("valkey: invalid params (1 violation(s)): Addr: rule \"required\" on value "),
		},
		{
			name: "whitespace addr rejected",
			params: valkey.ValkeyParams{
				Addr: "   ",
			},
			expectedResult: false,
			expectedError:  errors.New("valkey: addr is required"),
		},
		{
			name: "invalid negative db rejected",
			params: valkey.ValkeyParams{
				Addr: "127.0.0.1:6379",
				DB:   -1,
			},
			expectedResult: false,
			expectedError:  errors.New("valkey: invalid params (1 violation(s)): DB: rule \"gte\" on value -1"),
		},
		{
			name: "invalid high db rejected",
			params: valkey.ValkeyParams{
				Addr: "127.0.0.1:6379",
				DB:   16,
			},
			expectedResult: false,
			expectedError:  errors.New("valkey: invalid params (1 violation(s)): DB: rule \"lte\" on value 16"),
		},
		{
			name: "with tls enabled",
			params: valkey.ValkeyParams{
				Addr:   "127.0.0.1:6379",
				UseTLS: true,
			},
			expectedResult: true,
			expectedError:  nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
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
		c             *valkey.ValkeyClient
		ctx           context.Context
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "nil receiver rejected",
			c:             nil,
			ctx:           context.Background(),
			expectedError: errors.New("valkey: client is not initialized"),
		},
		{
			name:          "uninitialized internal client rejected",
			c:             &valkey.ValkeyClient{},
			ctx:           context.Background(),
			expectedError: errors.New("valkey: client is not initialized"),
		},
		{
			name: "canceled context aborts ping",
			c: func() *valkey.ValkeyClient {
				client, err := valkey.NewValkeyClient(valkey.ValkeyParams{Addr: "127.0.0.1:6379"})
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
		c              *valkey.ValkeyClient
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
			expectedError:  errors.New("valkey: client is not initialized"),
		},
		{
			name:           "uninitialized internal client rejected",
			c:              &valkey.ValkeyClient{},
			ctx:            context.Background(),
			key:            "key-1",
			expectedResult: nil,
			expectedError:  errors.New("valkey: client is not initialized"),
		},
		{
			name: "empty key rejected",
			c: func() *valkey.ValkeyClient {
				client, err := valkey.NewValkeyClient(valkey.ValkeyParams{Addr: "127.0.0.1:6379"})
				if err != nil {
					panic(err)
				}
				return client
			}(),
			ctx:            context.Background(),
			key:            "",
			expectedResult: nil,
			expectedError:  errors.New("valkey: key is required"),
		},
		{
			name: "canceled context aborts get",
			c: func() *valkey.ValkeyClient {
				client, err := valkey.NewValkeyClient(valkey.ValkeyParams{Addr: "127.0.0.1:6379"})
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
		c             *valkey.ValkeyClient
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
			expectedError: errors.New("valkey: client is not initialized"),
		},
		{
			name:          "uninitialized internal client rejected",
			c:             &valkey.ValkeyClient{},
			ctx:           context.Background(),
			key:           "k",
			value:         []byte(`x`),
			ttl:           time.Minute,
			expectedError: errors.New("valkey: client is not initialized"),
		},
		{
			name: "empty key rejected",
			c: func() *valkey.ValkeyClient {
				client, err := valkey.NewValkeyClient(valkey.ValkeyParams{Addr: "127.0.0.1:6379"})
				if err != nil {
					panic(err)
				}
				return client
			}(),
			ctx:           context.Background(),
			key:           "",
			value:         []byte(`x`),
			ttl:           time.Minute,
			expectedError: errors.New("valkey: key is required"),
		},
		{
			name: "zero ttl rejected",
			c: func() *valkey.ValkeyClient {
				client, err := valkey.NewValkeyClient(valkey.ValkeyParams{Addr: "127.0.0.1:6379"})
				if err != nil {
					panic(err)
				}
				return client
			}(),
			ctx:           context.Background(),
			key:           "k",
			value:         []byte(`x`),
			ttl:           0,
			expectedError: errors.New("valkey: ttl must be positive"),
		},
		{
			name: "negative ttl rejected",
			c: func() *valkey.ValkeyClient {
				client, err := valkey.NewValkeyClient(valkey.ValkeyParams{Addr: "127.0.0.1:6379"})
				if err != nil {
					panic(err)
				}
				return client
			}(),
			ctx:           context.Background(),
			key:           "k",
			value:         []byte(`x`),
			ttl:           -time.Second,
			expectedError: errors.New("valkey: ttl must be positive"),
		},
		{
			name: "canceled context aborts set",
			c: func() *valkey.ValkeyClient {
				client, err := valkey.NewValkeyClient(valkey.ValkeyParams{Addr: "127.0.0.1:6379"})
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
		c             *valkey.ValkeyClient
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
			expectedError: errors.New("valkey: client is not initialized"),
		},
		{
			name:          "uninitialized internal client rejected",
			c:             &valkey.ValkeyClient{},
			ctx:           context.Background(),
			key:           "k",
			expectedError: errors.New("valkey: client is not initialized"),
		},
		{
			name: "empty key rejected",
			c: func() *valkey.ValkeyClient {
				client, err := valkey.NewValkeyClient(valkey.ValkeyParams{Addr: "127.0.0.1:6379"})
				if err != nil {
					panic(err)
				}
				return client
			}(),
			ctx:           context.Background(),
			key:           "",
			expectedError: errors.New("valkey: key is required"),
		},
		{
			name: "canceled context aborts delete",
			c: func() *valkey.ValkeyClient {
				client, err := valkey.NewValkeyClient(valkey.ValkeyParams{Addr: "127.0.0.1:6379"})
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
		c              *valkey.ValkeyClient
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
			expectedError:  errors.New("valkey: client is not initialized"),
		},
		{
			name:           "uninitialized internal client rejected",
			c:              &valkey.ValkeyClient{},
			ctx:            context.Background(),
			key:            "k",
			expectedResult: 0,
			expectedError:  errors.New("valkey: client is not initialized"),
		},
		{
			name: "empty key rejected",
			c: func() *valkey.ValkeyClient {
				client, err := valkey.NewValkeyClient(valkey.ValkeyParams{Addr: "127.0.0.1:6379"})
				if err != nil {
					panic(err)
				}
				return client
			}(),
			ctx:            context.Background(),
			key:            "",
			expectedResult: 0,
			expectedError:  errors.New("valkey: key is required"),
		},
		{
			name: "canceled context aborts ttl",
			c: func() *valkey.ValkeyClient {
				client, err := valkey.NewValkeyClient(valkey.ValkeyParams{Addr: "127.0.0.1:6379"})
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
		c              *valkey.ValkeyClient
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
			expectedError:  errors.New("valkey: client is not initialized"),
		},
		{
			name:           "uninitialized internal client rejected",
			c:              &valkey.ValkeyClient{},
			ctx:            context.Background(),
			key:            "k",
			value:          []byte(`x`),
			ttl:            time.Minute,
			expectedResult: false,
			expectedError:  errors.New("valkey: client is not initialized"),
		},
		{
			name: "empty key rejected",
			c: func() *valkey.ValkeyClient {
				client, err := valkey.NewValkeyClient(valkey.ValkeyParams{Addr: "127.0.0.1:6379"})
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
			expectedError:  errors.New("valkey: key is required"),
		},
		{
			name: "zero ttl rejected",
			c: func() *valkey.ValkeyClient {
				client, err := valkey.NewValkeyClient(valkey.ValkeyParams{Addr: "127.0.0.1:6379"})
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
			expectedError:  errors.New("valkey: ttl must be positive"),
		},
		{
			name: "negative ttl rejected",
			c: func() *valkey.ValkeyClient {
				client, err := valkey.NewValkeyClient(valkey.ValkeyParams{Addr: "127.0.0.1:6379"})
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
			expectedError:  errors.New("valkey: ttl must be positive"),
		},
		{
			name: "canceled context aborts setnx",
			c: func() *valkey.ValkeyClient {
				client, err := valkey.NewValkeyClient(valkey.ValkeyParams{Addr: "127.0.0.1:6379"})
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
		c              *valkey.ValkeyClient
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
			expectedError:  errors.New("valkey: client is not initialized"),
		},
		{
			name:           "uninitialized internal client rejected",
			c:              &valkey.ValkeyClient{},
			ctx:            context.Background(),
			script:         "return 1",
			keys:           []string{"k"},
			args:           nil,
			expectedResult: nil,
			expectedError:  errors.New("valkey: client is not initialized"),
		},
		{
			name: "canceled context aborts eval",
			c: func() *valkey.ValkeyClient {
				client, err := valkey.NewValkeyClient(valkey.ValkeyParams{Addr: "127.0.0.1:6379"})
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
		c             *valkey.ValkeyClient
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
			c:             &valkey.ValkeyClient{},
			expectedError: nil,
		},
		{
			name: "initialized client closes cleanly",
			c: func() *valkey.ValkeyClient {
				client, err := valkey.NewValkeyClient(valkey.ValkeyParams{Addr: "127.0.0.1:6379"})
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
			err := tc.c.Close()
			if tc.expectedError != nil {
				assert.EqualError(t, err, tc.expectedError.Error())
				return
			}
			assert.NoError(t, err)
		})
	}
}
