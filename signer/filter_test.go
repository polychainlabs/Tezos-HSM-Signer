package signer

import (
	"math/big"
	"testing"
)

func testFilterOperation() *OperationFilter {
	return &OperationFilter{
		EnableTx: false,
		TxMaxFee: big.NewInt(-1),
	}

}

func TestFilterTxAllowed(t *testing.T) {
	filter := testFilterOperation()
	op, _ := ParseOperation([]byte(testP256Tx.Operation))

	if filter.IsAllowed(op) {
		t.Error("Transfer should not be allowed with Tx Disabled")
	}

	filter.EnableTx = true
	if !filter.IsAllowed(op) {
		t.Error("Transfer be allowed with Tx Disabled")
	}
}

func TestFilterFees(t *testing.T) {
	filter := testFilterOperation()
	op, _ := ParseOperation([]byte(testP256Tx.Operation))

	filter.EnableTx = true

	filter.TxMaxFee = ParseUXTZ("-1")
	if !filter.IsAllowed(op) {
		t.Error("Any fees should be allowed for subzero filter")
	}

	filter.TxMaxFee = ParseUXTZ("0")
	if filter.IsAllowed(op) {
		t.Error("Fees above TxMaxFee should not be allowed")
	}

	filter.TxMaxFee = ParseUXTZ("1")
	if !filter.IsAllowed(op) {
		t.Error("Fees below TxMaxFee should be allowed")
	}
}
