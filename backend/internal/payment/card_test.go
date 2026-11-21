package payment

import "testing"

func TestValidateTestCard(t *testing.T) {
	if err := Validate(Input{Number: "1111 1111 1111 1111", Exp: TestExp, CVC: TestCVC}); err != nil {
		t.Fatal(err)
	}
}

func TestValidateDecline(t *testing.T) {
	if err := Validate(Input{Number: FailNumber, Exp: TestExp, CVC: TestCVC}); err != ErrDeclined {
		t.Fatalf("err = %v", err)
	}
}

func TestValidateRejectsUnknown(t *testing.T) {
	if err := Validate(Input{Number: "4111111111111111", Exp: TestExp, CVC: TestCVC}); err != ErrInvalidCard {
		t.Fatalf("err = %v", err)
	}
}

func TestLast4(t *testing.T) {
	if got := Last4("1111-1111-1111-1111"); got != "1111" {
		t.Fatalf("got %s", got)
	}
}
