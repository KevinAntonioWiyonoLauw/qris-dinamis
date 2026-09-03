package qris

import (
	"encoding/hex"
	"strings"
)

// CalculateCRC16 returns the uppercase 4-digit hex CRC16-CCITT checksum
// (polynomial 0x1021, init 0xFFFF) of str — the EMVCo QR checksum.
func CalculateCRC16(str string) string {
	crc := uint16(0xffff)
	for i := 0; i < len(str); i++ {
		crc ^= uint16(str[i]) << 8
		for j := 0; j < 8; j++ {
			if crc&0x8000 != 0 {
				crc = (crc << 1) ^ 0x1021
			} else {
				crc <<= 1
			}
		}
	}
	// 16-bit mask; Go's (crc<<1)^0x1021 stays in uint16 so no explicit mask needed.
	return strings.ToUpper(hex.EncodeToString([]byte{byte(crc >> 8), byte(crc)}))
}
