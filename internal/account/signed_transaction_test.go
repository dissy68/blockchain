package account

import (
	"ledger/internal/signature"
	"math/big"
	"testing"
)

func TestSignedTransactionSignature(t *testing.T) {
	kp1 := signature.DefaultKeyGen()
	kp2 := signature.DefaultKeyGen()
	tx := NewSignedTransaction(kp1.Pk.Encode(), kp2.Pk.Encode(), 100, kp1.Sk)
	ser := Serialize(tx)
	sig := new(big.Int)
	sig.SetString(tx.Signature, 10)
	if !signature.Verify(ser, sig, kp1.Pk) {
		t.Errorf("Failed to verify signed transaction signature")
	}
}

func TestSignedTransactionExecution(t *testing.T) {
	kp1 := signature.DefaultKeyGen()
	kp2 := signature.DefaultKeyGen()
	ledger := MakeLedger()
	ledger.Accounts[kp1.Pk.Encode()] = 100
	ledger.Accounts[kp2.Pk.Encode()] = 50
	tx := NewSignedTransaction(kp1.Pk.Encode(), kp2.Pk.Encode(), 100, kp1.Sk)
	if err := ledger.ExecuteSignedTransaction(tx); err != nil {
		t.Errorf("Failed to execute signed transaction: %v", err)
	}
	if ledger.Accounts[kp1.Pk.Encode()] != 0 || ledger.Accounts[kp2.Pk.Encode()] != 150 {
		t.Errorf("Ledger accounts not updated correctly after transaction")
	}
}

func TestRejectInvalidTransaction(t *testing.T) {
	kp1 := signature.DefaultKeyGen()
	kp2 := signature.DefaultKeyGen()
	wrongKp := signature.DefaultKeyGen()
	ledger := MakeLedger()
	ledger.Accounts[kp1.Pk.Encode()] = 50
	ledger.Accounts[kp2.Pk.Encode()] = 50
	tx := NewSignedTransaction(kp1.Pk.Encode(), kp2.Pk.Encode(), 30, wrongKp.Sk)
	if err := ledger.ExecuteSignedTransaction(tx); err == nil {
		t.Errorf("Expected error when executing transaction with invalid signature")
	}
}
