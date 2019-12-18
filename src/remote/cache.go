package remote

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"path"

	"github.com/golang/protobuf/proto"

	pb "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"
	"github.com/thought-machine/please/src/core"
)

const cacheFile = "plz-out/log/remote_action_cache"

// Shutdown shuts down this remote client.
// In our case it writes the local action cache out to a file.
func (c *Client) Shutdown() {
	if c.state.Config.Remote.CacheActions {
		if err := c.writeCache(); err != nil {
			log.Error("Failed to write remote execution action cache: %s", err)
		}
	}
}

// writeCache writes the cache file.
func (c *Client) writeCache() error {
	if err := os.MkdirAll(path.Dir(cacheFile), core.DirPermissions); err != nil {
		return err
	}
	f, err := os.Create(cacheFile)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(c.configHash()); err != nil {
		return err
	}
	c.outputMutex.Lock()
	defer c.outputMutex.Unlock()
	// This is obviously the fastest way of writing, but we might want to do them in order
	// that we encountered or similar in order to get the most relevant ones first.
	b := []byte{0, 0, 0, 0}
	for label, dir := range c.outputs {
		if s := label.String(); len(s) < math.MaxUint16 { // Just ignore anything too long (shouldn't happen anyway)
			// We need to size-delimit the items we write since they are not self-describing.
			binary.LittleEndian.PutUint16(b, uint16(len(s)))
			if _, err := f.Write(b[:2]); err != nil {
				return err
			} else if _, err := f.Write([]byte(s)); err != nil {
				return err
			}
			msg, _ := proto.Marshal(dir)
			binary.LittleEndian.PutUint32(b, uint32(len(msg)))
			if _, err := f.Write(b); err != nil {
				return err
			} else if _, err := f.Write(msg); err != nil {
				return err
			}
		}
	}
	return nil
}

// readCache reads the cache file back again.
// Note that this must be tolerant to any kind of error that it encounters; it cannot assume
// well-formedness since we don't know the previous process wrote it successfully.
func (c *Client) readCache() error {
	f, err := os.Open(cacheFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // this is allowed
		}
		return err
	}
	defer f.Close()
	b := [sha1.Size]byte{}
	if _, err := f.Read(b[:]); err != nil {
		return err
	} else if !bytes.Equal(b[:], c.configHash()) {
		return fmt.Errorf("config hash doesn't match")
	}
	b2 := b[:2]  // 2 bytes for reading uint16s
	b4 := b[4:8] // 4 bytes for reading uint32s
	var buf bytes.Buffer
	for {
		if _, err := f.Read(b2); err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		size := int(binary.LittleEndian.Uint16(b2))
		buf.Reset()
		buf.Grow(size)
		if _, err := io.CopyN(&buf, f, int64(size)); err != nil {
			return err
		} else if _, err := f.Read(b4); err != nil {
			return err
		}
		label, err := core.TryParseBuildLabel(buf.String(), "", "")
		if err != nil {
			return err // We could continue here but there is probably no point
		}
		size = int(binary.LittleEndian.Uint32(b4))
		buf.Reset()
		buf.Grow(size)
		if _, err := io.CopyN(&buf, f, int64(size)); err != nil {
			return err
		}
		dir := &pb.Directory{}
		if err := proto.Unmarshal(buf.Bytes(), dir); err != nil {
			return err // Similarly here
		}
		c.outputMutex.Lock()
		c.outputs[label] = dir
		c.outputMutex.Unlock()
	}
}

// configHash returns a hash of the relevant config fields.
func (c *Client) configHash() []byte {
	h := sha1.New()
	h.Write([]byte(c.state.Config.Remote.URL))
	h.Write([]byte(c.state.Config.Remote.CASURL))
	h.Write([]byte(c.state.Config.Remote.Instance))
	h.Write([]byte(c.state.Config.Remote.HomeDir))
	return h.Sum(nil)
}
