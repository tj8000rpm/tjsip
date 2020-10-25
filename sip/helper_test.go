package sip

import (
	"fmt"
	"testing"
)

func TestGenerateBranchParamStartWith(t *testing.T) {
	generated := GenerateBranchParam()
	if generated[:7] != "z9hG4bK" {
		fmt.Printf("%s", generated)
		t.Errorf("Via branch parameter must be start with 'z9hG4bK': but '%v'", generated)
	}
}

func TestGenerateBranchParamLength(t *testing.T) {
	actual := len(GenerateBranchParam())
	expected := 7 + TagLenghtWithoutMagicCookie
	if actual != expected {
		t.Errorf("Via branch parameter length must be %v : but %v", expected, actual)
	}
}

func TestIsValidBarnchParam(t *testing.T) {
	if !IsValidBarnchParam("z9hG4bKaaaaaaa") || IsValidBarnchParam("zvvv9hG4bKaaaaaaa") {
		t.Errorf("Via branch parameter checker was broken")
	}
}

func TestGenerateTag(t *testing.T) {
	for i := 0; i < 10000; i++ {
		if GenerateTag() == GenerateTag() {
			t.Errorf("Tag generator was broken")
		}
	}
}
