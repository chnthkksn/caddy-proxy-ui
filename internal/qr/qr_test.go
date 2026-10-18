package qr

import (
	"bytes"
	"strings"
	"testing"
)

// The worked 1-M "HELLO WORLD" example from the QR spec tutorials: these
// data codewords must produce exactly these 10 ECC codewords.
func TestReedSolomon(t *testing.T) {
	data := []byte{32, 91, 11, 120, 209, 114, 220, 77, 67, 64, 236, 17, 236, 17, 236, 17}
	want := []byte{196, 35, 39, 119, 235, 215, 231, 226, 93, 23}
	if got := rsRemainder(data, rsDivisor(10)); !bytes.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestVersionSelection(t *testing.T) {
	cases := []struct {
		n, size int
	}{
		{14, 21},  // v1-M holds 14 bytes
		{15, 25},  // ...so 15 needs v2
		{106, 41}, // v6-M holds 106
		{107, 45}, // v7
		{213, 57}, // v10-M holds 213
	}
	for _, c := range cases {
		code, err := Encode(bytes.Repeat([]byte("a"), c.n))
		if err != nil {
			t.Fatalf("%d bytes: %v", c.n, err)
		}
		if code.Size != c.size {
			t.Errorf("%d bytes: size %d, want %d", c.n, code.Size, c.size)
		}
	}
	if _, err := Encode(bytes.Repeat([]byte("a"), 214)); err != ErrTooLong {
		t.Errorf("214 bytes: want ErrTooLong, got %v", err)
	}
}

func TestFinderPatternsAndPath(t *testing.T) {
	code, err := Encode([]byte("otpauth://totp/caddy-ui:admin?secret=ABC"))
	if err != nil {
		t.Fatal(err)
	}
	// Top-left finder: dark outer ring, light ring, dark 3×3 core.
	m := code.Modules
	if !m[0][0] || !m[0][6] || !m[6][0] || m[1][1] || !m[3][3] || m[7][7] {
		t.Error("top-left finder pattern looks wrong")
	}
	if p := code.SVGPath(); !strings.HasPrefix(p, "M4 4h1v1h-1z") {
		t.Errorf("path should start at the quiet-zone offset, got %.20q", p)
	}
}
