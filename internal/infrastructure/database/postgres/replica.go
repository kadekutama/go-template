package postgres

import (
	"fmt"

	"gorm.io/gorm"
)

// Pools carries the primary and replica handles. Writes and decision reads
// use Primary; cacheable reads may use Replica within the lag bound.
type Pools struct {
	Primary *gorm.DB
	Replica *gorm.DB
}

// PoolsParams carries constructor dependencies.
type PoolsParams struct {
	Primary *gorm.DB
	Replica *gorm.DB
}

// NewPools builds the pool set; Primary must be non-nil (Replica optional).
func NewPools(params PoolsParams) (*Pools, error) {
	if params.Primary == nil {
		return nil, fmt.Errorf("postgres: replica pools need a primary")
	}

	return &Pools{Primary: params.Primary, Replica: params.Replica}, nil
}

// LagSource reports replication lag in seconds. Unknown/unreadable lag fails
// toward the primary (safe direction).
type LagSource interface {
	// Sample returns current replica lag in seconds.
	Sample() (float64, error)
}

// Selector routes reads by lag and always writes to the primary.
type Selector struct {
	pools    *Pools
	lagBound float64
	source   LagSource
}

// SelectorParams carries constructor dependencies.
type SelectorParams struct {
	Pools *Pools
	// LagBound is the staleness bound in seconds; lag above it reads primary.
	LagBound float64
	// Source samples replica lag; nil means always-primary.
	Source LagSource
}

// NewSelector builds the router; Pools must be non-nil, LagBound positive.
func NewSelector(params SelectorParams) (*Selector, error) {
	if params.Pools == nil {
		return nil, fmt.Errorf("postgres: selector needs pools")
	}

	if params.LagBound <= 0 {
		return nil, fmt.Errorf("postgres: lag bound must be positive")
	}

	return &Selector{pools: params.Pools, lagBound: params.LagBound, source: params.Source}, nil
}

// Write always returns the primary.
func (s *Selector) Write() *gorm.DB { return s.pools.Primary }

// Read returns the replica when lag is within bound, else the primary.
// Decision reads must use Write; this is for cacheable reads only.
func (s *Selector) Read() *gorm.DB {
	if s.pools.Replica == nil || s.source == nil {
		return s.pools.Primary
	}

	lag, err := s.source.Sample()
	if err != nil || lag > s.lagBound {
		return s.pools.Primary
	}

	return s.pools.Replica
}
