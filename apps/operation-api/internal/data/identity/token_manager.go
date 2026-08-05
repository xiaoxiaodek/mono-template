package identitydata

import (
	bizidentity "github.com/vort-ads/vort-ads-template/apps/control-api/internal/biz/identity"
	"github.com/vort-ads/vort-ads-template/apps/internal/platform/security"
)

var _ bizidentity.TokenManager = TokenManager{}

type TokenManager struct{ manager security.JWTManager }

func NewTokenManager(manager security.JWTManager) TokenManager { return TokenManager{manager: manager} }
func (m TokenManager) SignAccessToken(principal bizidentity.Principal) (string, error) {
	return m.manager.SignAccessToken(toPlatformPrincipal(principal))
}
func (m TokenManager) SignRefreshToken(principal bizidentity.Principal) (string, error) {
	return m.manager.SignRefreshToken(toPlatformPrincipal(principal))
}
func (m TokenManager) VerifyAccessToken(token string) (bizidentity.TokenClaims, error) {
	claims, err := m.manager.VerifyAccessToken(token)
	if err != nil {
		return bizidentity.TokenClaims{}, err
	}
	return toBizClaims(claims), nil
}
func (m TokenManager) VerifyRefreshToken(token string) (bizidentity.TokenClaims, error) {
	claims, err := m.manager.VerifyRefreshToken(token)
	if err != nil {
		return bizidentity.TokenClaims{}, err
	}
	return toBizClaims(claims), nil
}
func toPlatformPrincipal(principal bizidentity.Principal) security.Principal {
	return security.Principal{UserID: principal.UserID, Email: principal.Email, Roles: append([]string(nil), principal.Roles...), Permissions: append([]string(nil), principal.Permissions...)}
}
func toBizClaims(claims *security.Claims) bizidentity.TokenClaims {
	if claims == nil {
		return bizidentity.TokenClaims{}
	}
	result := bizidentity.TokenClaims{UserID: claims.UserID}
	if claims.ExpiresAt != nil {
		result.ExpiresAt = claims.ExpiresAt.Time
	}
	return result
}
