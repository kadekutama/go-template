package hybrid_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"

	"github.com/kadekutama/go-template/internal/infrastructure/cache/hybrid"
	"github.com/kadekutama/go-template/internal/infrastructure/cache/store"
	mockstore "github.com/kadekutama/go-template/test/mock/store"
)

// txView is a second projection type used to prove shared-engine binding.
type txView struct {
	ID string
}

func TestTyped(t *testing.T) {
	t.Parallel()

	t.Run("views bind to the shared engine", func(t *testing.T) {
		shared := mockstore.NewMockExpiringStore(t)
		engine, err := hybrid.New(hybrid.Params{
			L1:            shared,
			L2:            shared,
			L1PopulateTTL: time.Minute,
			Codec:         hybrid.JSONCodec{},
		})
		require.NoError(t, err)

		balances := hybrid.Typed[cacheBalance](engine)
		require.NotNil(t, balances)

		transfers := hybrid.Typed[txView](engine)
		require.NotNil(t, transfers)
	})

	t.Run("nil engine view reports not initialized", func(t *testing.T) {
		var engine *hybrid.Cache

		view := hybrid.Typed[cacheBalance](engine)
		require.NotNil(t, view)

		_, err := view.Get(t.Context(), "k")
		assert.ErrorIs(t, err, hybrid.ErrNotInitialized)
	})

	t.Run("seam conformance for both adapters", func(t *testing.T) {
		var _ store.Store = (*mockstore.MockStore)(nil)
		var _ store.ExpiringStore = (*mockstore.MockExpiringStore)(nil)
	})
}

// TestModule proves the fx module wires *Cache and port.Cache from Params
// and that the container validates without cycles.
func TestModule(t *testing.T) {
	t.Parallel()

	err := fx.ValidateApp(
		hybrid.Module(),
		fx.Provide(func() hybrid.Params {
			return hybrid.Params{
				L1:            &mockstore.MockStore{},
				L1PopulateTTL: time.Minute,
				Codec:         hybrid.JSONCodec{},
			}
		}),
	)
	require.NoError(t, err)
}
