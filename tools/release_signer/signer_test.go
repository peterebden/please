package signer

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/openpgp"
)

const (
	pubKey = "tools/release_signer/test_data/pub.gpg"
	secKey = "tools/release_signer/test_data/sec.gpg"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func verifyFile(signed, signature, keyring string) bool {
	f1, err := os.Open(signed)
	must(err)
	f2, err := os.Open(signature)
	must(err)
	f3, err := os.Open(keyring)
	must(err)
	entities, err := openpgp.ReadArmoredKeyRing(f3)
	must(err)
	_, err = openpgp.CheckArmoredDetachedSignature(entities, f1, f2)
	return err == nil
}

func TestSignFile(t *testing.T) {
	assert.NoError(t, SignFile("tools/release_signer/test_data/test.txt", "test.txt.asc", secKey, "test@please.build", "testtest"))
	assert.True(t, verifyFile("tools/release_signer/test_data/test.txt", "test.txt.asc", pubKey))
}

func TestSignFileBadPassphrase(t *testing.T) {
	assert.Error(t, SignFile("tools/release_signer/test_data/test.txt", "test.txt.asc", secKey, "test@please.build", "nope"))
}

func TestSignFileBadSignature(t *testing.T) {
	assert.NoError(t, SignFile("tools/release_signer/test_data/test.txt", "test.txt.asc", secKey, "test@please.build", "testtest"))
	assert.False(t, verifyFile("tools/release_signer/test_data/bad.txt", "test.txt.asc", pubKey))
}
