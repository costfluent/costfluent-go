package costfluent

// ProviderStatus is where a provider connection stands, spelled as the Public API sends it. A new
// status may appear, so a client should tolerate one it does not list.
// TestProviderStatusesMatchTheContract holds this list to the API's declared values.
type ProviderStatus string

const (
	ProviderStatusPending    ProviderStatus = "pending"
	ProviderStatusConnecting ProviderStatus = "connecting"
	ProviderStatusActive     ProviderStatus = "active"
	ProviderStatusSyncing    ProviderStatus = "syncing"
	ProviderStatusError      ProviderStatus = "error"
	ProviderStatusDisabled   ProviderStatus = "disabled"
)

// ProviderStatuses returns every status the Public API declares, in the contract's order.
func ProviderStatuses() []ProviderStatus {
	return []ProviderStatus{
		ProviderStatusPending,
		ProviderStatusConnecting,
		ProviderStatusActive,
		ProviderStatusSyncing,
		ProviderStatusError,
		ProviderStatusDisabled,
	}
}
