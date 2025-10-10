package types

// IsAbstractAccount checks if this vesting account is an abstract account in dchain's abstractaccount antehandler.
// Abstract vesting accounts are identified by having a 32-byte address (wasm contract address)
// instead of the standard 20-byte SDK address.
// This method enables ContinuousVestingAccount to implement the AbstractAccountI
// interface defined in the abstractaccount module.
// Note other modules might have this address length, i.e. group accounts but we expect no transactions will be signed directly.
func (cva ContinuousVestingAccount) IsAbstractAccount() bool {
	addr := cva.GetAddress()
	if addr == nil {
		return false
	}
	// Wasm contract addresses are 32 bytes, while regular SDK addresses are 20 bytes
	return len(addr) == 32
}
