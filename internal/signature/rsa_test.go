package signature

import "testing"

func TestSignature(t *testing.T) {
	kp := DefaultKeyGen()
	message := []byte("Hello, World!")
	signature := Sign(message, kp.Sk)
	if !Verify(message, signature, kp.Pk) {
		t.Errorf("Failed to verify signature")
	}
}

func TestLongSignature(t *testing.T) {
	kp := KeyGen(DEFAULT_KEY_SIZE * 2)
	message := []byte("This is a longer message to test longer RSA signature length on longer message.")
	signature := Sign(message, kp.Sk)
	if !Verify(message, signature, kp.Pk) {
		t.Errorf("Failed to verify signature")
	}
}
