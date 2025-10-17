package account

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"ledger/internal/signature"
	"math/big"

	"github.com/google/uuid"
)

type SignedTransaction struct {
	ID        string
	From      string
	To        string
	Amount    int
	Signature string
}

func NewSignedTransaction(from, to string, amount int, sk *signature.SecretKey) *SignedTransaction {
	id := uuid.New().String()
	tx := &SignedTransaction{
		ID:     id,
		From:   from,
		To:     to,
		Amount: amount,
	}
	ser := Serialize(tx)
	sig := signature.Sign(ser, sk).String()
	tx.Signature = sig

	return tx
}

func AddStr(b *bytes.Buffer, s string) error {
	err := binary.Write(b, binary.BigEndian, uint32(len(s)))
	if err != nil {
		return err
	}
	b.WriteString(s)
	return nil
}

func AddInt(b *bytes.Buffer, val int) error {
	err := binary.Write(b, binary.BigEndian, int64(val))
	return err
}

// Does not include the Signature, meant to be used for signing
// [ID][From][To][Amount]
func Serialize(tx *SignedTransaction) []byte {
	var b bytes.Buffer

	AddStr(&b, tx.ID)
	AddStr(&b, tx.From)
	AddStr(&b, tx.To)
	AddInt(&b, tx.Amount)

	return b.Bytes()
}

func (tx *SignedTransaction) Verify() bool {
	ser := Serialize(tx)
	sig := new(big.Int)
	sig.SetString(tx.Signature, 10)
	pk, err := signature.DecodePk(tx.From)
	if err != nil {
		return false
	}
	return signature.Verify(ser, sig, pk)
}

func (l *Ledger) ExecuteSignedTransaction(tx *SignedTransaction) error {
	// Check signature
	l.lock.Lock()
	defer l.lock.Unlock()

	if !tx.Verify() {
		// invalid signature
		return fmt.Errorf("invalid signature for transaction %s", tx.ID)
	}

	l.Accounts[tx.From] -= tx.Amount
	l.Accounts[tx.To] += tx.Amount

	return nil
}
