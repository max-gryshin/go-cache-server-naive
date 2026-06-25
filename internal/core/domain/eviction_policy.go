package domain

type EvictionPolicy int

const (
	EvictionPolicyLRU EvictionPolicy = iota
)
