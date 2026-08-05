package identitydata_test

import (
	"testing"
	"time"

	bizidentity "github.com/vort-ads/vort-ads-template/apps/operation-api/internal/biz/identity"
	identitydata "github.com/vort-ads/vort-ads-template/apps/operation-api/internal/data/identity"
	"github.com/vort-ads/vort-ads-template/internal/platform/security"
)

func TestTokenManagerCrossVerifiesAndPreservesPrincipal(t *testing.T) {
	platform := security.NewJWTManager("test-secret-with-enough-length", time.Minute, time.Hour)
	adapter := identitydata.NewTokenManager(platform)
	principal := bizidentity.Principal{UserID: "usr_1", Email: "a@example.com", Roles: []string{"admin"}, Permissions: []string{"write"}}
	access, err := adapter.SignAccessToken(principal)
	if err != nil {
		t.Fatal(err)
	}
	platformClaims, err := platform.VerifyAccessToken(access)
	if err != nil {
		t.Fatal(err)
	}
	if platformClaims.Roles[0] != "admin" || platformClaims.Permissions[0] != "write" {
		t.Fatalf("claims = %+v", platformClaims)
	}
	refresh, err := platform.SignRefreshToken(security.Principal{UserID: principal.UserID, Email: principal.Email, Roles: principal.Roles, Permissions: principal.Permissions})
	if err != nil {
		t.Fatal(err)
	}
	claims, err := adapter.VerifyRefreshToken(refresh)
	if err != nil {
		t.Fatal(err)
	}
	if claims.UserID != "usr_1" || claims.ExpiresAt.IsZero() {
		t.Fatalf("claims = %+v", claims)
	}
}

func TestTokenManagerRejectsWrongTokenTypes(t *testing.T) {
	adapter := identitydata.NewTokenManager(security.NewJWTManager("test-secret-with-enough-length", time.Minute, time.Hour))
	principal := bizidentity.Principal{UserID: "usr_1"}
	access, _ := adapter.SignAccessToken(principal)
	refresh, _ := adapter.SignRefreshToken(principal)
	if _, err := adapter.VerifyRefreshToken(access); err == nil {
		t.Fatal("access token accepted as refresh")
	}
	if _, err := adapter.VerifyAccessToken(refresh); err == nil {
		t.Fatal("refresh token accepted as access")
	}
}
