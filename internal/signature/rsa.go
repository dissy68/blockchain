package signature

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"math/big"
	"strings"
)

const DEFAULT_KEY_SIZE = 512

type SecretKey struct {
	n *big.Int
	d *big.Int
}

type PublicKey struct {
	n *big.Int
	e *big.Int
}

type KeyPair struct {
	Sk *SecretKey
	Pk *PublicKey
}

func NewSecretKey(n *big.Int, d *big.Int) *SecretKey {
	sk := new(SecretKey)
	sk.n = n
	sk.d = d
	return sk
}

func NewPublicKey(n *big.Int, e *big.Int) *PublicKey {
	pk := new(PublicKey)
	pk.n = n
	pk.e = e
	return pk
}

func NewKeyPair(sk *SecretKey, pk *PublicKey) *KeyPair {
	kp := new(KeyPair)
	kp.Sk = sk
	kp.Pk = pk
	return kp
}

func KeyGen(k int) *KeyPair {
	var p *big.Int
	var q *big.Int
	var err error
	var d *big.Int
	e := big.NewInt(3)

	for {
		p, err = rand.Prime(rand.Reader, k/2)

		if err != nil {
			panic(err)
		}

		pm1 := new(big.Int).Sub(p, big.NewInt(1))
		gcd_p := new(big.Int).GCD(nil, nil, e, pm1)
		condition_p := (gcd_p.Cmp(big.NewInt(1)) == 0)

		if !condition_p {
			continue
		}

		q, err = rand.Prime(rand.Reader, k/2)

		if err != nil {
			panic(err)
		}

		// gcd condition
		qm1 := new(big.Int).Sub(q, big.NewInt(1))
		gcd_q := new(big.Int).GCD(nil, nil, e, qm1)
		condition_q := (gcd_q.Cmp(big.NewInt(1)) == 0)

		if !condition_q {
			continue
		}

		// d condition
		mult := new(big.Int).Mul(pm1, qm1) // (p -1)*(q-1)
		d = new(big.Int).ModInverse(e, mult)
		condition_d := (d != nil)

		// all conditions true -> found fitting p and q
		if condition_d {
			break
		}
	}
	// make and return keypair
	n := new(big.Int).Mul(p, q)
	pk := NewPublicKey(n, e)
	sk := NewSecretKey(n, d)
	keyPair := NewKeyPair(sk, pk)
	return keyPair
}

func DefaultKeyGen() *KeyPair {
	return KeyGen(DEFAULT_KEY_SIZE)
}

func (p *PublicKey) Encode() string {
	nBytes := p.n.Bytes()
	eBytes := p.e.Bytes()

	nEnc := base64.StdEncoding.EncodeToString(nBytes)
	eEnc := base64.StdEncoding.EncodeToString(eBytes)
	return nEnc + "." + eEnc
}

func Decode(encodedPk string) (*PublicKey, error) {
	parts := strings.Split(encodedPk, ".")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid encoded key, not exactly one '.'(dot)")
	}

	nBytes, err := base64.StdEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, err
	}
	eBytes, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, err
	}

	n := new(big.Int).SetBytes(nBytes)
	e := new(big.Int).SetBytes(eBytes)
	return NewPublicKey(n, e), nil
}

func Sign(message []byte, sk *SecretKey) *big.Int {
	h := sha256.Sum256(message)
	mh := new(big.Int).SetBytes(h[:])
	return new(big.Int).Exp(mh, sk.d, sk.n)
}

func Verify(message []byte, sig *big.Int, pk *PublicKey) bool {
	h := sha256.Sum256(message)
	mh := new(big.Int).SetBytes(h[:])
	check := new(big.Int).Exp(sig, pk.e, pk.n)
	return mh.Cmp(check) == 0
}
