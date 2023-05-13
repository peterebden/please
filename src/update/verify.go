package update

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	_ "embed" // needed for //go:embed
	"encoding/hex"
	"fmt"
	"io"
	"strings"

	"github.com/thought-machine/please/src/cli"
	"github.com/thought-machine/please/src/plugin"
)

// pubkey is the public key we verify Please releases with.
//
//go:embed key.pub
var key []byte

// verifyDownload fetches a detached signature for a download and verifies it's OK.
// It returns a reader to the verified content.
func verifyDownload(message io.Reader, url string, progress bool) io.Reader {
	signature := mustDownload(url+".sig", false)
	defer signature.Close()
	return mustVerifySignature(message, signature, progress)
}

// mustVerifySignature verifies an OpenPGP detached signature of a file.
// It panics if the signature is not correct.
// On success it returns an equivalent reader to the original.
func mustVerifySignature(message, signature io.Reader, progress bool) io.Reader {
	// We need to be able to reuse the body again afterwards so we have to
	// download the original into a buffer.
	b, err := io.ReadAll(message)
	if err != nil {
		panic(err)
	}
	log.Notice("Verifying signature of downloaded tarball...")
	verify := plugin.MustLoadSymbol[func(io.Reader, io.Reader, []byte) bool]("verify", "VerifySignature")
	if !verify(bytes.NewReader(b), signature, key) {
		panic("Invalid signature on downloaded file, possible tampering; will not continue.")
	}
	if progress {
		return bufio.NewReader(cli.NewProgressReader(io.NopCloser(bytes.NewReader(b)), len(b), "Extracting"))
	}
	return bufio.NewReader(io.NopCloser(bytes.NewReader(b)))
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
