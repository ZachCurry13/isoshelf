package sniff

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"
	"time"
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
		if got.Kind != tt.want {
			t.Errorf("%s: got %s, want %s", tt.name, got.Kind, tt.want)
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

// pvd builds an image whose primary volume descriptor carries these fields.
func pvd(label, system, publisher, application, created string) []byte {
	b := at(40000, map[int][]byte{
		pvdOffset:      {1},
		pvdOffset + 1:  []byte("CD001"),
		pvdOffset + 8:  []byte(system),
		pvdOffset + 40: []byte(label),
		// Fields are space-padded on real images; NUL padding happens too.
		pvdOffset + 318: []byte(publisher),
		pvdOffset + 574: []byte(application),
		pvdOffset + 813: []byte(created),
	})
	return b
}

func TestReadVolume(t *testing.T) {
	// Field widths, and a label padded with spaces as the standard says.
	info, err := Detect(bytes.NewReader(pvd(
		"Ubuntu 24.04.2 LTS amd64        ", "LINUX", "CANONICAL", "", "2025021309411500\x00")), 40000)
	if err != nil {
		t.Fatal(err)
	}
	if info.Kind != ISO {
		t.Fatalf("kind = %s, want %s", info.Kind, ISO)
	}
	want := Volume{
		Label:   "Ubuntu 24.04.2 LTS amd64",
		System:  "LINUX",
		Created: time.Date(2025, 2, 13, 9, 41, 15, 0, time.UTC),
	}
	want.Publisher = "CANONICAL"
	if info.Volume != want {
		t.Errorf("volume = %+v, want %+v", info.Volume, want)
	}
	if got := info.Volume.Year(); got != "2025" {
		t.Errorf("year = %q, want 2025", got)
	}
}

func TestReadVolumeOddities(t *testing.T) {
	// A time zone offset, as FreeBSD and TrueNAS images carry: -20 quarter
	// hours is five hours behind GMT, so the date is five hours later in UTC.
	info, _ := Detect(bytes.NewReader(pvd("TRUENAS", "FreeBSD", "", "", "2023112018531100\xec")), 40000)
	if got, want := info.Volume.Created, time.Date(2023, 11, 20, 23, 53, 11, 0, time.UTC); !got.Equal(want) {
		t.Errorf("created = %s, want %s", got, want)
	}

	// An unset date leaves the field empty instead of reporting year zero.
	info, _ = Detect(bytes.NewReader(pvd("SOMEDISC", "", "", "", "0000000000000000\x00")), 40000)
	if !info.Volume.Created.IsZero() {
		t.Errorf("created = %s, want no time", info.Volume.Created)
	}
	if got := info.Volume.Year(); got != "" {
		t.Errorf("year = %q, want empty", got)
	}

	// A UDF-only image has no primary volume descriptor to read.
	info, _ = Detect(bytes.NewReader(at(40000, map[int][]byte{34817: []byte("NSR02")})), 40000)
	if info.Kind != ISO || !info.Volume.Empty() {
		t.Errorf("udf: kind %s, volume %+v", info.Kind, info.Volume)
	}
}

func TestVolumeSays(t *testing.T) {
	v := Volume{Label: "Kali Linux amd64 1", Publisher: "KALI", Preparer: "XORRISO-1.5.6", Application: "KALI LIVE"}
	got := v.Says()
	if !strings.Contains(got, "Kali Linux amd64 1") || !strings.Contains(got, "KALI LIVE") {
		t.Errorf("says = %q, want the label and application", got)
	}
	if strings.Contains(got, "XORRISO") {
		t.Errorf("says = %q, should leave out the build tool", got)
	}
}
