package tools

// Config aggregates per-tool configuration. Zero values pick sensible defaults.
type Config struct {
	// Search configures grep (and future search tools).
	Search SearchConfig
	// Exec configures bash (and node, once #81 lands).
	Exec ExecConfig
}
