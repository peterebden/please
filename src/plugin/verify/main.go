package main

import (
	"bytes"
	"crypto"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"strings"

	"github.com/sigstore/sigstore/pkg/cryptoutils"
	"github.com/sigstore/sigstore/pkg/signature"

	"github.com/thought-machine/please/src/cli/logging"
)

var log = logging.Log

// VerifySignature verifies an OpenPGP detached signature of a file.
// It returns true if the signature is correct according to the given key.
func VerifySignature(signed, sig io.Reader, key []byte) bool {
	pub, err := cryptoutils.UnmarshalPEMToPublicKey(key)
	if err != nil {
		log.Fatalf("err: %v", err)
	}

	verifier, err := signature.LoadVerifier(pub, crypto.SHA256)
	if err != nil {
		log.Fatalf("err: %v", err)
	}

	return verifier.VerifySignature(sig, signed) == nil
}

// mustVerifyHash verifies the sha256 hash of the downloaded file matches one of the given ones.
// On success it returns an equivalent reader, on failure it panics.
func mustVerifyHash(r io.Reader, hashes []string) io.Reader {
	b, err := io.ReadAll(r)
	if err != nil {
		panic(err)
	}
	log.Notice("Verifying hash of downloaded tarball")
	sum := sha256.Sum256(b)
	checksum := hex.EncodeToString(sum[:])
	for _, hash := range hashes {
		if hash == checksum {
			log.Notice("Good checksum: %s", checksum)
			return bytes.NewReader(b)
		}
	}
	if len(hashes) == 1 {
		panic(fmt.Errorf("Invalid checksum of downloaded file, was %s, expected %s", checksum, hashes[0]))
	}
	panic(fmt.Errorf("Invalid checksum of downloaded file, was %s, expected one of [%s]", checksum, strings.Join(hashes, ", ")))
}

func main() {}
