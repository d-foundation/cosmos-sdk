package types

import (
	"bytes"

	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/gogoproto/proto"
)

var (
	_ cryptotypes.PubKey = (*NilPubKey)(nil)
)

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

// GetPubKey returns the public key for this vesting account.
// For abstract accounts (32-byte addresses), this returns a NilPubKey,
// similar to how AbstractAccount.GetPubKey() works.
// This ensures that transactions built with ContinuousVestingAccount abstract accounts
// include a NilPubKey instead of nil, preventing panics in GetSigningTxData().
func (cva ContinuousVestingAccount) GetPubKey() cryptotypes.PubKey {
	// If this is an abstract account (32-byte address), return NilPubKey
	if cva.IsAbstractAccount() {
		pk := NewNilPubKey(cva.GetAddress())
		return pk
	}
	// Otherwise, delegate to the base account
	return cva.BaseVestingAccount.BaseAccount.GetPubKey()
}

// ---------------------------- NilPubKey ---------------------------------

func NewNilPubKey(bz []byte) *NilPubKey {
	return &NilPubKey{AddressBytes: bz}
}

func (pk *NilPubKey) Address() cryptotypes.Address {
	return cryptotypes.Address(pk.AddressBytes)
}

func (pk *NilPubKey) Bytes() []byte {
	return nil
}

func (pk *NilPubKey) VerifySignature(_ []byte, _ []byte) bool {
	panic("NilPubKey.VerifySignature should never be invoked")
}

func (pk *NilPubKey) Equals(other cryptotypes.PubKey) bool {
	otherPk, ok := other.(*NilPubKey)
	if !ok {
		return false
	}

	return bytes.Equal(pk.AddressBytes, otherPk.AddressBytes)
}

func (pk *NilPubKey) Type() string {
	return "/" + proto.MessageName(pk)
}

func (pk *NilPubKey) String() string {
	return sdk.AccAddress(pk.AddressBytes).String()
}
