package account

import (
	"strconv"
	"testing"
)

func TestTransactions(t *testing.T) {
	tx1 := &Transaction{
		ID:     "tx1",
		From:   "Alice",
		To:     "Bob",
		Amount: 100,
	}
	tx2 := &Transaction{
		ID:     "tx2",
		From:   "Alice",
		To:     "Bob",
		Amount: 100,
	}
	l1 := MakeLedger()
	l2 := MakeLedger()

	for i := 0; i < 10; i++ {
		tx1.ID = "tx1_" + strconv.Itoa(i)
		tx2.ID = "tx1_" + strconv.Itoa(i)
		l1.Transaction(tx1)
		l2.Transaction(tx1)
		l1.Transaction(tx2)
		l2.Transaction(tx2)
	}

	if !VerifyLedgerConsistency([]*Ledger{l1, l2}) {
		t.Errorf("Ledgers are inconsistent")
	}
}
