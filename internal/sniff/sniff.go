// Package sniff identifies image files by their content, never by their
// extension.
package sniff

import (
	"bytes"
	"encoding/binary"
	"io"
	"os"
)

// Kind is what a file's content looks like.
type Kind string

const (
	Unknown  Kind = "unknown"
	ISO      Kind = "iso"    // ISO 9660 or UDF optical image
	RawCD    Kind = "raw-cd" // raw 2352-byte CD sectors, as in .bin/.cue
	Disk     Kind = "disk"   // MBR or GPT disk image
	WIM      Kind = "wim"    // Windows Imaging Format
	VHD      Kind = "vhd"
	VHDX     Kind = "vhdx"
	EFI      Kind = "efi" // PE executable, such as an EFI application
	XZ       Kind = "xz"
	Gzip     Kind = "gzip"
	Zip      Kind = "zip"
	SevenZip Kind = "7z"
)

// IsArchive reports whether k is a compressed file or archive that has to be
// extracted before it can boot.
func (k Kind) IsArchive() bool {
	switch k {
	case XZ, Gzip, Zip, SevenZip:
		return true
	}
	return false
}

// headSize is how much of the start of a file is read. It covers the optical
// volume descriptors in sectors 16 to 18.
const headSize = 64 << 10

var (
	cdSync      = []byte{0x00, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0x00}
	opticalIDs  = []string{"CD001", "BEA01", "NSR02", "NSR03", "TEA01"}
	vhdCookie   = []byte("conectix")
	prefixMagic = []struct {
		magic []byte
		kind  Kind
	}{
		{[]byte("vhdxfile"), VHDX},
		{[]byte("MSWIM\x00\x00\x00"), WIM},
		{[]byte{0xFD, '7', 'z', 'X', 'Z', 0x00}, XZ},
		{[]byte{'7', 'z', 0xBC, 0xAF, 0x27, 0x1C}, SevenZip},
		{[]byte{0x1F, 0x8B}, Gzip},
		{[]byte("PK\x03\x04"), Zip},
	}
)

// File detects the kind of the file at path.
func File(path string) (Kind, error) {
	f, err := os.Open(path)
	if err != nil {
		return Unknown, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return Unknown, err
	}
	return Detect(f, info.Size())
}

// Detect reads the start of r, and the end for VHD footers, and reports what
// the content looks like.
func Detect(r io.ReaderAt, size int64) (Kind, error) {
	head := make([]byte, min(size, headSize))
	if _, err := r.ReadAt(head, 0); err != nil && err != io.EOF {
		return Unknown, err
	}

	for _, m := range prefixMagic {
		if bytes.HasPrefix(head, m.magic) {
			return m.kind, nil
		}
	}
	// Optical images come before disk images: hybrid ISOs also carry an MBR.
	if isOptical(head) {
		return ISO, nil
	}
	if bytes.HasPrefix(head, cdSync) {
		return RawCD, nil
	}
	if bytes.HasPrefix(head, vhdCookie) {
		return VHD, nil // dynamic VHD: a copy of the footer starts the file
	}
	if size >= 512 {
		footer := make([]byte, len(vhdCookie))
		if _, err := r.ReadAt(footer, size-512); err != nil && err != io.EOF {
			return Unknown, err
		}
		if bytes.Equal(footer, vhdCookie) {
			return VHD, nil // fixed VHD: raw disk followed by a 512-byte footer
		}
	}
	if isPE(head) {
		return EFI, nil
	}
	if len(head) >= 520 && string(head[512:520]) == "EFI PART" {
		return Disk, nil // GPT header in sector 1
	}
	if len(head) >= 512 && head[510] == 0x55 && head[511] == 0xAA {
		return Disk, nil // MBR boot signature
	}
	return Unknown, nil
}

// isOptical looks for an ISO 9660 or UDF volume descriptor in sectors 16-18.
func isOptical(head []byte) bool {
	for sector := 16; sector <= 18; sector++ {
		off := sector*2048 + 1
		if off+5 > len(head) {
			return false
		}
		for _, id := range opticalIDs {
			if string(head[off:off+5]) == id {
				return true
			}
		}
	}
	return false
}

// isPE checks for an MZ header that points at a "PE\0\0" signature.
func isPE(head []byte) bool {
	if len(head) < 0x40 || head[0] != 'M' || head[1] != 'Z' {
		return false
	}
	off := int(binary.LittleEndian.Uint32(head[0x3C:]))
	return off >= 0x40 && off+4 <= len(head) && string(head[off:off+4]) == "PE\x00\x00"
}
