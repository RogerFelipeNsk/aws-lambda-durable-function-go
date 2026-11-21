package payment

import (
	"errors"
	"strings"
	"unicode"
)

const (
	TestNumber = "1111111111111111"
	TestExp    = "01/2040"
	TestCVC    = "111"
	FailNumber = "0000000000000000"
)

var (
	ErrInvalidCard = errors.New("dados do cartão inválidos")
	ErrDeclined    = errors.New("cartão recusado")
)

type Input struct {
	Number string
	Exp    string
	CVC    string
}

func Normalize(n string) string {
	var b strings.Builder
	for _, r := range n {
		if unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func Last4(n string) string {
	digits := Normalize(n)
	if len(digits) < 4 {
		return digits
	}
	return digits[len(digits)-4:]
}

func Validate(in Input) error {
	number := Normalize(in.Number)
	exp := strings.TrimSpace(in.Exp)
	cvc := strings.TrimSpace(in.CVC)
	if number == FailNumber && exp == TestExp && cvc == TestCVC {
		return ErrDeclined
	}
	if number == TestNumber && exp == TestExp && cvc == TestCVC {
		return nil
	}
	return ErrInvalidCard
}
