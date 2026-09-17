package sniff

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// at returns size zero bytes with each pattern written at its offset.
func at(size int, patterns map[int][]byte) []byte {
	b := make([]byte, size)
	for off, p := range patterns {
		copy(b[off:], p)
	}
	return b
}

func TestDetect(t *testing.T) {
	mbr := []byte{0x55, 0xAA}
	pe := at(0x100, map[int][]byte{0: []byte("MZ"), 0x80: []byte("PE\x00\x00")})
	binary.LittleEndian.PutUint32(pe[0x3C:], 0x80)

	tests := []struct {
		name string
		data []byte
		want Kind
	}{
		{"empty", nil, Unknown},
		{"text", []byte("just some notes\n"), Unknown},
		{"iso 9660", at(40000, map[int][]byte{32769: []byte("CD001")}), ISO},
		{"hybrid iso with mbr", at(40000, map[int][]byte{510: mbr, 32769: []byte("CD001")}), ISO},
		{"udf only", at(40000, map[int][]byte{34817: []byte("NSR02")}), ISO},
		{"iso too short", at(32000, map[int][]byte{510: mbr}), Disk},
		{"raw cd sectors", at(2352, map[int][]byte{0: cdSync}), RawCD},
		{"gpt disk", at(1024, map[int][]byte{510: mbr, 512: []byte("EFI PART")}), Disk},
		{"mbr disk", at(1024, map[int][]byte{510: mbr}), Disk},
		{"fixed vhd", at(2048, map[int][]byte{510: mbr, 1536: []byte("conectix")}), VHD},
		{"dynamic vhd", at(2048, map[int][]byte{0: []byte("conectix")}), VHD},
		{"vhdx", at(1024, map[int][]byte{0: []byte("vhdxfile")}), VHDX},
		{"wim", at(1024, map[int][]byte{0: []byte("MSWIM\x00\x00\x00")}), WIM},
		{"efi application", pe, EFI},
		{"mz without pe header", at(0x100, map[int][]byte{0: []byte("MZ")}), Unknown},
		{"xz", []byte{0xFD, '7', 'z', 'X', 'Z', 0x00, 1, 2}, XZ},
		{"gzip", []byte{0x1F, 0x8B, 0x08, 0x00}, Gzip},
		{"zip", []byte("PK\x03\x04rest"), Zip},
		{"7z", []byte{'7', 'z', 0xBC, 0xAF, 0x27, 0x1C, 0, 4}, SevenZip},
	}
	for _, tt := range tests {
		got, err := Detect(bytes.NewReader(tt.data), int64(len(tt.data)))
		if err != nil {
			t.Errorf("%s: %v", tt.name, err)
			continue
		}
		if got != tt.want {
			t.Errorf("%s: got %s, want %s", tt.name, got, tt.want)
		}
	}
}

func TestIsArchive(t *testing.T) {
	for _, k := range []Kind{XZ, Gzip, Zip, SevenZip} {
		if !k.IsArchive() {
			t.Errorf("%s.IsArchive() = false", k)
		}
	}
	for _, k := range []Kind{ISO, Disk, RawCD, Unknown} {
		if k.IsArchive() {
			t.Errorf("%s.IsArchive() = true", k)
		}
	}
}
