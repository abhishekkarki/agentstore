package agentstore

// StoreOption configures the store.
type StoreOption func(*storeConfig)

type storeConfig struct {
	// Reducer used for state materialization. Defaults to DefaultReducer().
	reducer Reducer

	// SnapshotInterval is the number of events between automatic snapshots.
	// Set to 0 to disable auto-snapshots. Default: 100.
	snapshotInterval uint64

	// InMemory uses an in-memory backend instead of Pebble.
	// Useful for testing or ephemeral sessions.
	inMemory bool
}

func defaultConfig() *storeConfig {
	return &storeConfig{
		reducer:          DefaultReducer(),
		snapshotInterval: 100,
	}
}

// WithReducer sets a custom reducer for state materialization.
func WithReducer(r Reducer) StoreOption {
	return func(c *storeConfig) {
		c.reducer = r
	}
}

// WithSnapshotInterval sets how often automatic snapshots are created.
// A value of 0 disables automatic snapshots.
func WithSnapshotInterval(n uint64) StoreOption {
	return func(c *storeConfig) {
		c.snapshotInterval = n
	}
}

// WithInMemory uses an in-memory storage backend.
// Data is lost when the store is closed.
func WithInMemory() StoreOption {
	return func(c *storeConfig) {
		c.inMemory = true
	}
}

// ListOption configures session listing.
type ListOption func(*listConfig)

type listConfig struct {
	limit  int
	offset int
	label  string
	value  string
}

func defaultListConfig() *listConfig {
	return &listConfig{
		limit: 100,
	}
}

// WithLimit sets the maximum number of sessions to return.
func WithLimit(n int) ListOption {
	return func(c *listConfig) {
		c.limit = n
	}
}

// WithOffset sets the starting offset for pagination.
func WithOffset(n int) ListOption {
	return func(c *listConfig) {
		c.offset = n
	}
}

// WithLabelFilter filters sessions that have the given label key-value pair.
func WithLabelFilter(key, value string) ListOption {
	return func(c *listConfig) {
		c.label = key
		c.value = value
	}
}
