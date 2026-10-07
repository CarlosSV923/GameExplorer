package infrastructure_test

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/argon2"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/identity/domain"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/identity/infrastructure"
)

func TestArgon2idRoundTrip(t *testing.T) {
	t.Parallel()

	phc, err := infrastructure.HashPassword("correct horse")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(phc, "$argon2id$v=19$m=19456,t=2,p=1$") {
		t.Fatalf("unexpected PHC format: %s", phc)
	}
	v, err := infrastructure.NewArgon2idVerifier(phc)
	if err != nil {
		t.Fatal(err)
	}
	if !v.Verify("correct horse") {
		t.Error("correct password rejected")
	}
	if v.Verify("correct horsE") {
		t.Error("wrong password accepted")
	}

	other, _ := infrastructure.HashPassword("correct horse")
	if other == phc {
		t.Error("two hashes of the same password must differ (random salt)")
	}
}

func TestArgon2idVerifiesHashesWithOtherParameters(t *testing.T) {
	t.Parallel()

	// A hash made with the phase-1 parameters (m=64 MiB, t=3, p=2) must keep
	// working after the defaults changed: verification reads them from the hash.
	salt := []byte("0123456789abcdef")
	key := argon2.IDKey([]byte("legacy"), salt, 3, 64*1024, 2, 32)
	enc := base64.RawStdEncoding
	legacy := fmt.Sprintf("$argon2id$v=19$m=65536,t=3,p=2$%s$%s", enc.EncodeToString(salt), enc.EncodeToString(key))

	v, err := infrastructure.NewArgon2idVerifier(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if !v.Verify("legacy") || v.Verify("other") {
		t.Fatal("hash with older parameters must verify exactly")
	}
}

func TestArgon2idFromPlain(t *testing.T) {
	t.Parallel()

	v, err := infrastructure.NewArgon2idVerifierFromPlain("pw")
	if err != nil || !v.Verify("pw") || v.Verify("nope") {
		t.Fatalf("plain verifier broken: %v", err)
	}
}

func TestArgon2idRejectsMalformedHash(t *testing.T) {
	t.Parallel()

	for _, bad := range []string{"", "plain", "$argon2i$v=19$m=1,t=1,p=1$c2FsdA$a2V5", "$argon2id$v=18$m=1,t=1,p=1$c2FsdA$a2V5", "$argon2id$v=19$x$c2FsdA$a2V5", "$argon2id$v=19$m=1,t=1,p=1$!!$a2V5"} {
		if _, err := infrastructure.NewArgon2idVerifier(bad); !errors.Is(err, infrastructure.ErrInvalidHash) {
			t.Errorf("%q: err = %v, want ErrInvalidHash", bad, err)
		}
	}
}

func TestHMACTokenCodec(t *testing.T) {
	t.Parallel()

	codec := infrastructure.NewHMACTokenCodec([]byte(strings.Repeat("k", 32)))
	s := domain.NewSession(time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC), time.Hour)

	token := codec.Encode(s)
	got, err := codec.Decode(token)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if !got.IssuedAt.Equal(s.IssuedAt) || !got.ExpiresAt.Equal(s.ExpiresAt) {
		t.Fatalf("round trip = %+v, want %+v", got, s)
	}

	other := infrastructure.NewHMACTokenCodec([]byte(strings.Repeat("z", 32)))
	payload, sig, _ := strings.Cut(token, ".")
	tampered := payload + "x." + sig
	for name, tok := range map[string]string{
		"other secret": other.Encode(s),
		"tampered":     tampered,
		"no dot":       "abc",
		"not base64":   "!!.!!",
	} {
		if _, err := codec.Decode(tok); !errors.Is(err, domain.ErrInvalidSession) {
			t.Errorf("%s: err = %v, want ErrInvalidSession", name, err)
		}
	}
}

func TestMemoryThrottle(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	th := infrastructure.NewMemoryThrottle(time.Minute, 3, func() time.Time { return now })

	for i := range 3 {
		if !th.Allow("a") {
			t.Fatalf("attempt %d rejected within burst", i+1)
		}
	}
	if th.Allow("a") {
		t.Fatal("4th attempt allowed, want throttled")
	}
	if !th.Allow("b") {
		t.Fatal("another client must not be affected")
	}
	now = now.Add(time.Minute)
	if !th.Allow("a") {
		t.Fatal("token not refilled after a minute")
	}
}
