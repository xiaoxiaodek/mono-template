package security

import (
	"testing"
	"time"
)

func BenchmarkJWTVerifyAccessToken(b *testing.B) {
	manager := NewJWTManager("benchmark-secret-with-enough-length", 15*time.Minute, time.Hour)
	token, err := manager.SignAccessToken(Principal{
		UserID:      "usr_benchmark",
		Email:       "benchmark@example.com",
		Roles:       []string{"admin"},
		Permissions: []string{"ads:read", "ads:write"},
	})
	if err != nil {
		b.Fatalf("sign access token: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := manager.VerifyAccessToken(token); err != nil {
			b.Fatalf("verify access token: %v", err)
		}
	}
}
