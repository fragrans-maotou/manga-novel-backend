package service

import "strconv"

// ImageProxyBufferSize is the fixed stream buffer size required by SPEC §6.3.
const ImageProxyBufferSize = 32 * 1024

// ApplyXORInPlace XORs every byte in buf with key (single-byte cipher).
func ApplyXORInPlace(buf []byte, key byte) {
	if key == 0 || len(buf) == 0 {
		return
	}
	for i := range buf {
		buf[i] ^= key
	}
}

// ParseXORKey parses the proxy ticket xor_key field (decimal string of one byte).
// Empty string means no encryption and returns (0, false, nil).
func ParseXORKey(raw string) (key byte, enabled bool, err error) {
	if raw == "" {
		return 0, false, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, false, err
	}
	if n < 0 || n > 255 {
		return 0, false, strconv.ErrRange
	}
	return byte(n), true, nil
}
