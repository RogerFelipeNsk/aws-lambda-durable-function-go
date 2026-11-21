package pix

import (
	"fmt"
	"strings"
)

const (
	gui     = "br.gov.bcb.pix"
	crcPoly = 0x1021
)

func CopiaECola(txid, name, city, chave string, amountCents int64) string {
	amount := fmt.Sprintf("%d.%02d", amountCents/100, amountCents%100)
	merchant := tlv("00", gui) + tlv("01", chave)
	additional := tlv("05", txid)

	var b strings.Builder
	b.WriteString(tlv("00", "01"))
	b.WriteString(tlv("26", merchant))
	b.WriteString(tlv("52", "0000"))
	b.WriteString(tlv("53", "986"))
	b.WriteString(tlv("54", amount))
	b.WriteString(tlv("58", "BR"))
	b.WriteString(tlv("59", clip(name, 25)))
	b.WriteString(tlv("60", clip(city, 15)))
	b.WriteString(tlv("62", additional))
	payload := b.String() + "6304"
	return payload + crc16(payload)
}

func tlv(id, value string) string {
	return fmt.Sprintf("%s%02d%s", id, len(value), value)
}

func clip(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	if s == "" {
		return "NA"
	}
	return s
}

func crc16(s string) string {
	crc := uint16(0xFFFF)
	for i := range len(s) {
		crc ^= uint16(s[i]) << 8
		for range 8 {
			if crc&0x8000 != 0 {
				crc = (crc << 1) ^ crcPoly
			} else {
				crc <<= 1
			}
		}
	}
	return fmt.Sprintf("%04X", crc)
}
