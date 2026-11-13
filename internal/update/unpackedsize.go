package update

import (
	"bytes"
	"encoding/binary"
	"os"

	"github.com/ZachCurry13/isoshelf/internal/sniff"
)

// unpackedSize is how big a compressed file will be once unpacked, read
// from the file's own trailer so the free-space check can happen before
// anything is written. 0 means it can't tell, and the extract still stops
// cleanly if the disk fills.
func unpackedSize(full string, kind sniff.Kind) int64 {
	f, err := os.Open(full)
	if err != nil {
		return 0
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return 0
	}
	switch kind {
	case sniff.Gzip:
		return gzipSize(f, info.Size())
	case sniff.XZ:
		return xzSize(f, info.Size())
	}
	return 0
}

// gzipSize reads the last four bytes, which hold the size modulo 4 GB. A
// compressed file is never bigger than what is in it, so the smallest size
// with those low bits that is at least the compressed size is the guess.
func gzipSize(f *os.File, size int64) int64 {
	var tail [4]byte
	if size < 18 {
		return 0
	}
	if _, err := f.ReadAt(tail[:], size-4); err != nil {
		return 0
	}
	n := int64(binary.LittleEndian.Uint32(tail[:]))
	for n < size {
		n += 1 << 32
	}
	return n
}

// xzSize adds up the uncompressed sizes in the stream's index, which the
// footer says where to find. Only a file holding a single stream is read;
// anything else answers 0.
func xzSize(f *os.File, size int64) int64 {
	var foot [12]byte
	if size < 32 {
		return 0
	}
	if _, err := f.ReadAt(foot[:], size-12); err != nil || !bytes.Equal(foot[10:], []byte("YZ")) {
		return 0
	}
	indexSize := (int64(binary.LittleEndian.Uint32(foot[4:8])) + 1) * 4
	if indexSize > size-24 || indexSize > 64<<20 {
		return 0
	}
	index := make([]byte, indexSize)
	if _, err := f.ReadAt(index, size-12-indexSize); err != nil || index[0] != 0 {
		return 0
	}
	p := index[1:]
	records, ok := varint(&p)
	if !ok {
		return 0
	}
	var total int64
	for range records {
		if _, ok := varint(&p); !ok { // unpadded size
			return 0
		}
		u, ok := varint(&p)
		if !ok {
			return 0
		}
		total += int64(u)
	}
	return total
}

// varint reads one of xz's multibyte integers and moves p past it.
func varint(p *[]byte) (uint64, bool) {
	var v uint64
	for i, b := range *p {
		if i >= 9 {
			return 0, false
		}
		v |= uint64(b&0x7f) << (7 * i)
		if b&0x80 == 0 {
			*p = (*p)[i+1:]
			return v, true
		}
	}
	return 0, false
}
