package github

import (
	"testing"
	"time"
)

func TestSignAndVerifyInstallState(t *testing.T) {
	state, err := SignInstallState("secret", "/projects/abc", time.Unix(1_700_000_000, 0))
	if err != nil {
		t.Fatalf("SignInstallState() = %v", err)
	}
	returnPath, err := VerifyInstallState("secret", state, time.Unix(1_700_000_000, 0))
	if err != nil {
		t.Fatalf("VerifyInstallState() = %v", err)
	}
	if returnPath != "/projects/abc" {
		t.Fatalf("return = %q, want /projects/abc", returnPath)
	}
}

func TestVerifyInstallStateExpired(t *testing.T) {
	state, err := SignInstallState("secret", "/projects/abc", time.Unix(1_700_000_000, 0))
	if err != nil {
		t.Fatalf("SignInstallState() = %v", err)
	}
	if _, err := VerifyInstallState("secret", state, time.Unix(1_700_010_000, 0)); err == nil {
		t.Fatal("VerifyInstallState() = nil, want error for expired state")
	}
}
