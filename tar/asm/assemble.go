package asm

import (
	"bytes"
	"fmt"
	"hash/crc64"
	"io"

	"github.com/vbatts/tar-split/tar/storage"
)

// NewOutputTarStream returns an io.ReadCloser that is an assemble tar archive
// stream.
//
// It takes a FileGetter, for mapping the file payloads that are to be read in,
// and a storage.Unpacker, which has access to the rawbytes and file order
// metadata. With the combination of these two items, a precise assembled Tar
// archive is possible.
func NewOutputTarStream(fg FileGetter, up storage.Unpacker) io.ReadCloser {
	// ... Since these are interfaces, this is possible, so let's not have a nil pointer
	if fg == nil || up == nil {
		return nil
	}
	pr, pw := io.Pipe()
	go func() {
		for {
			entry, err := up.Next()
			if err != nil {
				pw.CloseWithError(err)
				break
			}
			switch entry.Type {
			case storage.SegmentType:
				if _, err := pw.Write(entry.Payload); err != nil {
					pw.CloseWithError(err)
					break
				}
			case storage.FileType:
				if entry.Size == 0 {
					continue
				}
				fh, err := fg.Get(entry.Name)
				if err != nil {
					pw.CloseWithError(err)
					break
				}
				defer fh.Close()
				c := crc64.New(crcTable)
				tRdr := io.TeeReader(fh, c)
				if _, err := io.Copy(pw, tRdr); err != nil {
					pw.CloseWithError(err)
					break
				}
				if !bytes.Equal(c.Sum(nil), entry.Payload) {
					pw.CloseWithError(fmt.Errorf("file integrity checksum failed for %q", entry.Name))
					break
				}
			}
		}
	}()
	return pr
}
