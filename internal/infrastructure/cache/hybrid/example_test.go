package hybrid_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	appport "github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/service"
	"github.com/kadekutama/go-template/internal/infrastructure/cache/hybrid"
	"github.com/kadekutama/go-template/internal/infrastructure/cache/local"
)

// cacheBalance is an example structured value for a typed cache.
type cacheBalance struct {
	Minor int64 `json:"minor"`
}

// balanceReader is a typical service: it holds only the typed view and never
// touches L1/L2/codec. The L1/L2 instances are built once by the composition
// root and injected into the shared engine.
type balanceReader struct {
	balances *hybrid.TypedCache[cacheBalance]
}

// BalanceMinor reads one cached balance.
func (r balanceReader) BalanceMinor(ctx context.Context, key string) (int64, error) {
	balance, err := r.balances.Get(ctx, key)
	if err != nil {
		return 0, err
	}

	return balance.Minor, nil
}

// mustEngine builds the example L1-only engine, mirroring what the
// composition root wires once from configuration.
func mustEngine() *hybrid.Cache {
	l1, err := local.NewOtterCache(local.OtterParams{MaximumWeight: 1 << 20})
	if err != nil {
		panic(err)
	}

	engine, err := hybrid.New(hybrid.Params{L1: l1})
	if err != nil {
		panic(err)
	}

	return engine
}

// ExampleCache_typedView shows the intended split: wiring builds the shared
// engine once from injected L1/L2 instances, the projection owner binds one
// typed view per value type, and services only call Get/Set. Values cross
// the API as the declared type; conversion happens internally at the shared
// byte boundary only.
func ExampleCache_typedView() {
	engine := mustEngine()
	balances := hybrid.Typed[cacheBalance](engine)

	key, err := service.BuildBalanceKey("tenant-1", "account-1", "USD")
	if err != nil {
		panic(err)
	}

	ctx := context.Background()
	if err := balances.Set(ctx, key, cacheBalance{Minor: 4200}, hybrid.DefaultTTLs().Balance); err != nil {
		panic(err)
	}

	reader := balanceReader{balances: balances}

	minor, err := reader.BalanceMinor(ctx, key)
	if err != nil {
		panic(err)
	}

	fmt.Println(minor)
	// Output: 4200
}

// ExampleCache_negativeCaching shows the nil-value use case: caching
// "record does not exist" so repeated lookups do not hit the database. The
// returned error distinguishes a negative entry (nil error) from a miss
// (ErrCacheMiss).
func ExampleCache_negativeCaching() {
	engine := mustEngine()

	key, err := service.BuildBalanceKey("tenant-1", "account-missing", "USD")
	if err != nil {
		panic(err)
	}

	ctx := context.Background()

	// Store the negative result with a short TTL.
	if err := engine.Set(ctx, key, nil, time.Minute); err != nil {
		panic(err)
	}

	value, err := engine.Get(ctx, key)
	fmt.Println(err == nil, value == nil)
	// Output: true true
}

// ExampleCache_portCache shows the engine used through the application
// port: consumers depend on port.Cache, not on this adapter.
func ExampleCache_portCache() {
	engine := mustEngine()

	var cache appport.Cache = engine

	key, err := service.BuildBalanceCursorKey("tenant-1", "account-1", "USD", "42")
	if err != nil {
		panic(err)
	}

	ctx := context.Background()
	if err := cache.Set(ctx, key, []byte(`{"minor":9710}`), hybrid.DefaultTTLs().BalanceCursor); err != nil {
		panic(err)
	}

	value, err := cache.Get(ctx, key)
	fmt.Println(err == nil, string(value))
	// Output: true {"minor":9710}
}

func TestBalanceReader(t *testing.T) {
	t.Parallel()

	engine := mustEngine()
	balances := hybrid.Typed[cacheBalance](engine)

	key, err := service.BuildBalanceKey("tenant-1", "account-1", "USD")
	require.NoError(t, err)

	ctx := context.Background()
	require.NoError(t, balances.Set(ctx, key, cacheBalance{Minor: 4200}, hybrid.DefaultTTLs().Balance))

	reader := balanceReader{balances: balances}

	minor, err := reader.BalanceMinor(ctx, key)
	require.NoError(t, err)
	require.Equal(t, int64(4200), minor)
}
