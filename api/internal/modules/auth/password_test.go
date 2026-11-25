package auth

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

const fixturePassword = "Fixture-only-revision-login"

func TestHashPasswordUsesMeasuredProfile(t *testing.T) {
	hash, err := HashPassword(fixturePassword)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(hash, "argon2id$2$19456$1$") {
		t.Fatal("want Argon2id with 19 MiB, two passes, and one lane")
	}
	if !VerifyPassword(hash, fixturePassword) || VerifyPassword(hash, "wrong-password") {
		t.Fatal("new hash must accept only the correct password")
	}
}

func TestVerifyPasswordAcceptsLegacyProfile(t *testing.T) {
	const legacyHash = "argon2id$1$65536$4$uhsStRw4xZXleq0OJ/qHnQ$ToIObHsx1CZP6D0Ddjtp3y5EwOvrP6rmTCNwbNDrMwI"
	if !VerifyPassword(legacyHash, fixturePassword) || VerifyPassword(legacyHash, "wrong-password") {
		t.Fatal("legacy hash must accept only the correct password")
	}
}

func TestPasswordWorkWaitStopsWhenRequestExpires(t *testing.T) {
	service := NewService(nil, nil)
	for range cap(service.passwordWorkers) {
		service.passwordWorkers <- struct{}{}
	}

	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()
	if _, err := service.verifyPassword(ctx, "", fixturePassword); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("verification with all workers busy returned %v; want deadline exceeded", err)
	}
}
