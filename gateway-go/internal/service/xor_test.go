package service

import (
	"bytes"
	"testing"
)

func TestApplyXORInPlaceRoundTrip(t *testing.T) {
	plain := []byte("hello-manga-image")
	key := byte(12)
	buf := append([]byte(nil), plain...)
	ApplyXORInPlace(buf, key)
	if bytes.Equal(buf, plain) {
		t.Fatal("expected ciphertext to differ from plaintext")
	}
	ApplyXORInPlace(buf, key)
	if !bytes.Equal(buf, plain) {
		t.Fatalf("round-trip failed: got %q want %q", buf, plain)
	}
}

func TestApplyXORInPlaceKnownVector(t *testing.T) {
	// plaintext 0x00,0x01,0xFF with key 0x0C => 0x0C,0x0D,0xF3
	in := []byte{0x00, 0x01, 0xFF}
	ApplyXORInPlace(in, 0x0C)
	want := []byte{0x0C, 0x0D, 0xF3}
	if !bytes.Equal(in, want) {
		t.Fatalf("got %v want %v", in, want)
	}
}

func TestParseXORKey(t *testing.T) {
	k, on, err := ParseXORKey("")
	if err != nil || on || k != 0 {
		t.Fatalf("empty: k=%d on=%v err=%v", k, on, err)
	}
	k, on, err = ParseXORKey("12")
	if err != nil || !on || k != 12 {
		t.Fatalf("12: k=%d on=%v err=%v", k, on, err)
	}
	if _, _, err := ParseXORKey("256"); err == nil {
		t.Fatal("expected range error for 256")
	}
	if _, _, err := ParseXORKey("abc"); err == nil {
		t.Fatal("expected parse error for abc")
	}
}

func TestImageProxyBufferSize(t *testing.T) {
	if ImageProxyBufferSize != 32*1024 {
		t.Fatalf("ImageProxyBufferSize=%d want %d", ImageProxyBufferSize, 32*1024)
	}
}
