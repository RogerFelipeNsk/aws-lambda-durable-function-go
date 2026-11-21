package pix

import (
	"strings"
	"testing"
)

func TestCopiaEColaHasCRCAndAmount(t *testing.T) {
	code := CopiaECola("ord_test", "Arena Store Demo", "SAO PAULO", "arena@demo.local", 19990)
	if !strings.HasPrefix(code, "000201") {
		t.Fatalf("payload prefix = %q", code[:6])
	}
	if !strings.Contains(code, "5406199.90") {
		t.Fatalf("amount missing in %s", code)
	}
	if !strings.Contains(code, "br.gov.bcb.pix") {
		t.Fatalf("gui missing")
	}
	if got, want := code[len(code)-4:], crc16(code[:len(code)-4]); got != want {
		t.Fatalf("crc = %s want %s", got, want)
	}
}
