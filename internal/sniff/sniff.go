// Package sniff identifies image files by their content, never by their
// extension, and reads what an optical image says about itself.
package sniff

import (
	"bytes"
	"encoding/binary"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
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

// Info is what the start of a file says about it.
type Info struct {
	Kind Kind
	// Volume is filled for ISO 9660 images. UDF-only images leave it empty.
	Volume Volume
}

// Volume is what an ISO 9660 image says about itself, from the primary volume
// descriptor. Every field can be empty: plenty of images fill in nothing but
// a label, and some labels ("ISOIMAGE", "ESD_ISO") say nothing at all.
type Volume struct {
	// Label is the volume identifier, such as "Ubuntu 24.04.2 LTS amd64".
	Label string `json:"label,omitempty"`
	// System names the system that can boot the first sectors, such as
	// "LINUX" or "FreeBSD".
	System string `json:"system,omitempty"`
	// Publisher, Preparer and Application are set by whoever built the image.
	Publisher   string `json:"publisher,omitempty"`
	Preparer    string `json:"preparer,omitempty"`
	Application string `json:"application,omitempty"`
	// Created is when the image was built, as recorded inside it. Two copies
	// of one image carry the same time, however they were downloaded.
	Created time.Time `json:"created,omitzero"`
}

// Empty reports whether nothing was read.
func (v Volume) Empty() bool { return v == Volume{} }

// Says returns the text an image uses to describe itself, for matching
// against catalog entries. Build tools are left out: they say who made the
// image, not what it is.
func (v Volume) Says() string {
	return strings.Join([]string{v.Label, v.Publisher, v.Application}, " ")
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

// File reads the file at path and reports what it is.
func File(path string) (Info, error) {
	f, err := os.Open(path)
	if err != nil {
		return Info{Kind: Unknown}, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return Info{Kind: Unknown}, err
	}
	return Detect(f, info.Size())
}

// Detect reads the start of r, and the end for VHD footers, and reports what
// the content looks like.
func Detect(r io.ReaderAt, size int64) (Info, error) {
	head := make([]byte, min(size, headSize))
	if _, err := r.ReadAt(head, 0); err != nil && err != io.EOF {
		return Info{Kind: Unknown}, err
	}

	for _, m := range prefixMagic {
		if bytes.HasPrefix(head, m.magic) {
			return Info{Kind: m.kind}, nil
		}
	}
	// Optical images come before disk images: hybrid ISOs also carry an MBR.
	if isOptical(head) {
		return Info{Kind: ISO, Volume: readVolume(head)}, nil
	}
	if bytes.HasPrefix(head, cdSync) {
		return Info{Kind: RawCD}, nil
	}
	if bytes.HasPrefix(head, vhdCookie) {
		return Info{Kind: VHD}, nil // dynamic VHD: a copy of the footer starts the file
	}
	if size >= 512 {
		footer := make([]byte, len(vhdCookie))
		if _, err := r.ReadAt(footer, size-512); err != nil && err != io.EOF {
			return Info{Kind: Unknown}, err
		}
		if bytes.Equal(footer, vhdCookie) {
			return Info{Kind: VHD}, nil // fixed VHD: raw disk followed by a 512-byte footer
		}
	}
	if isPE(head) {
		return Info{Kind: EFI}, nil
	}
	if len(head) >= 520 && string(head[512:520]) == "EFI PART" {
		return Info{Kind: Disk}, nil // GPT header in sector 1
	}
	if len(head) >= 512 && head[510] == 0x55 && head[511] == 0xAA {
		return Info{Kind: Disk}, nil // MBR boot signature
	}
	return Info{Kind: Unknown}, nil
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

// The primary volume descriptor is sector 16 of an ISO 9660 image: type 1,
// then "CD001". Its fields are fixed-width and padded with spaces.
const pvdOffset = 16 * 2048

var pvdFields = []struct {
	get  func(*Volume) *string
	off  int
	size int
}{
	{func(v *Volume) *string { return &v.System }, 8, 32},
	{func(v *Volume) *string { return &v.Label }, 40, 32},
	{func(v *Volume) *string { return &v.Publisher }, 318, 128},
	{func(v *Volume) *string { return &v.Preparer }, 446, 128},
	{func(v *Volume) *string { return &v.Application }, 574, 128},
}

// readVolume reads the primary volume descriptor, if there is one.
func readVolume(head []byte) Volume {
	if len(head) < pvdOffset+881 {
		return Volume{}
	}
	pvd := head[pvdOffset:]
	if pvd[0] != 1 || string(pvd[1:6]) != "CD001" {
		return Volume{} // UDF-only image, or the descriptors are in another order
	}
	var v Volume
	for _, f := range pvdFields {
		*f.get(&v) = text(pvd[f.off : f.off+f.size])
	}
	v.Created = readTime(pvd[813 : 813+17])
	return v
}

// text trims a fixed-width field. Fields are space-padded, but images in the
// wild pad with NULs too.
func text(b []byte) string {
	return strings.TrimSpace(strings.TrimRight(string(b), "\x00"))
}

// readTime reads a 17-byte date: "YYYYMMDDHHMMSShh" plus a signed offset from
// GMT in quarter hours. All zeroes means the field was left unset.
func readTime(b []byte) time.Time {
	digits := string(b[:16])
	if strings.Trim(digits, "0") == "" || strings.Trim(digits, "0123456789") != "" {
		return time.Time{}
	}
	t, err := time.Parse("20060102150405", digits[:14])
	if err != nil {
		return time.Time{}
	}
	if offset := int(int8(b[16])); offset != 0 {
		t = t.Add(-time.Duration(offset) * 15 * time.Minute)
	}
	if t.Year() < 1980 || t.Year() > 2200 {
		return time.Time{} // a date that far out is a filler value
	}
	return t
}

// Year returns the year an image was built, or "" if it doesn't say. It is
// how a Windows image made in 2021 is told from one made in 2016.
func (v Volume) Year() string {
	if v.Created.IsZero() {
		return ""
	}
	return strconv.Itoa(v.Created.Year())
}

// isPE checks for an MZ header that points at a "PE\0\0" signature.
func isPE(head []byte) bool {
	if len(head) < 0x40 || head[0] != 'M' || head[1] != 'Z' {
		return false
	}
	off := int(binary.LittleEndian.Uint32(head[0x3C:]))
	return off >= 0x40 && off+4 <= len(head) && string(head[off:off+4]) == "PE\x00\x00"
}
