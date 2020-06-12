package signer

import (
	"log"
	"math/big"
)

// ParseUXTZ from a full XTZ string, returning a big integer of uXTZ or fail
func ParseUXTZ(xtzString string) *big.Int {
	uxtz, ok := new(big.Int).SetString(xtzString, 10)
	if !ok {
		log.Fatal("Could not set big.Int to string: ", xtzString)
	}
	// Convert from XTZ to uXTZ
	xtz := uxtz.Mul(uxtz, big.NewInt(1000000))
	return xtz
}
