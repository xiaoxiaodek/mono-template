package identityservice

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	bizidentity "github.com/vort-ads/vort-ads-template/apps/operation-api/internal/biz/identity"
	identitydata "github.com/vort-ads/vort-ads-template/apps/operation-api/internal/data/identity"
	"github.com/vort-ads/vort-ads-template/apps/operation-api/internal/data/identity/memory"
	"github.com/vort-ads/vort-ads-template/internal/middleware"
	"github.com/vort-ads/vort-ads-template/internal/platform/apperrors"
	"github.com/vort-ads/vort-ads-template/internal/platform/security"
	"github.com/vort-ads/vort-ads-template/pkg/idgen"
)

func TestRegisterRoutesRegistersExpectedPaths(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	NewHandler(nil).RegisterRoutes(router.Group("/api/v1"))

	got := make(map[string]bool)
	for _, route := range router.Routes() {
		got[route.Method+" "+route.Path] = true
	}
	want := map[string]bool{
		http.MethodPost + " /api/v1/auth/register": true,
		http.MethodPost + " /api/v1/auth/login":    true,
		http.MethodPost + " /api/v1/auth/refresh":  true,
		http.MethodGet + " /api/v1/me":             true,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("runtime routes = %#v, want %#v", got, want)
	}
}

func TestApplicationOutputsMapToHTTPDTOs(t *testing.T) {
	now := time.Now().UTC()
	user := bizidentity.UserOutput{
		ID: "usr_1", Email: "admin@example.com", Roles: []string{"admin"}, CreatedAt: now, UpdatedAt: now,
	}
	output := bizidentity.AuthOutput{User: user, AccessToken: "access", RefreshToken: "refresh"}
	wantUser := User{
		ID: "usr_1", Email: "admin@example.com", Roles: []string{"admin"}, CreatedAt: now, UpdatedAt: now,
	}
	wantAuth := AuthData{User: wantUser, AccessToken: "access", RefreshToken: "refresh"}
	if got := toUser(user); !reflect.DeepEqual(got, wantUser) {
		t.Fatalf("toUser = %#v, want %#v", got, wantUser)
	}
	if got := toAuthData(output); !reflect.DeepEqual(got, wantAuth) {
		t.Fatalf("toAuthData = %#v, want %#v", got, wantAuth)
	}
	mapped := toUser(user)
	mapped.Roles[0] = "mutated"
	if user.Roles[0] != "admin" {
		t.Fatalf("mapped roles mutated application output: %#v", user.Roles)
	}
}

func TestMeMiddlewareRunsAuthenticationBeforeAuthenticatedMiddlewareAndHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var order []string
	auth := func(c *gin.Context) {
		order = append(order, "auth")
		c.Set(middleware.PrincipalKey, security.Principal{UserID: "missing-user"})
		c.Next()
	}
	authenticated := func(c *gin.Context) {
		principal, ok := middleware.GetPrincipal(c)
		if !ok || principal.UserID != "missing-user" {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		order = append(order, "authenticated")
		c.Next()
	}
	manager := security.NewJWTManager("test-secret-with-enough-length", time.Minute, time.Hour)
	usecase := bizidentity.NewUsecase(
		memory.NewUserRepository(), memory.NewTokenStore(), security.BcryptPasswordHasher{Cost: 4},
		identitydata.NewTokenManager(manager), idgen.New, nil,
	)
	router := gin.New()
	NewHandler(usecase, auth).RegisterRoutes(router.Group("/api/v1"), authenticated)
	response := performJSONRequest(router, http.MethodGet, "/api/v1/me", "", "")
	if !reflect.DeepEqual(order, []string{"auth", "authenticated"}) {
		t.Fatalf("middleware order = %#v", order)
	}
	if response.Code != http.StatusNotFound {
		t.Fatalf("handler status = %d, want 404 proving handler ran; body=%s", response.Code, response.Body.String())
	}
}

func TestRegisterEndpointReturnsTokenPair(t *testing.T) {
	gin.SetMode(gin.TestMode)
	manager := security.NewJWTManager("test-secret-with-enough-length", time.Minute, time.Hour)
	usecase := bizidentity.NewUsecase(
		memory.NewUserRepository(),
		memory.NewTokenStore(),
		security.BcryptPasswordHasher{Cost: 10},
		identitydata.NewTokenManager(manager),
		idgen.New,
		nil,
	)
	handler := NewHandler(usecase)
	router := gin.New()
	router.Use(middleware.RequestID())
	handler.RegisterRoutes(router.Group("/api/v1"))

	body := bytes.NewBufferString(`{"email":"admin@example.com","password":"password123"}`)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", body)
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if !bytes.Contains(response.Body.Bytes(), []byte("access_token")) {
		t.Fatalf("response missing access_token: %s", response.Body.String())
	}
}

func TestRegisterEndpointRejectsShortPassword(t *testing.T) {
	router := newTestRouter(t)
	response := performJSONRequest(router, http.MethodPost, "/api/v1/auth/register", `{"email":"admin@example.com","password":"short"}`, "")

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	assertEnvelopeCode(t, response, "VALIDATION_ERROR")
}

func TestRegisterEndpointRejectsUnknownJSONFields(t *testing.T) {
	router := newTestRouter(t)
	response := performJSONRequest(router, http.MethodPost, "/api/v1/auth/register",
		`{"email":"admin@example.com","password":"password123","role":"admin"}`, "")

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	assertEnvelopeCode(t, response, "VALIDATION_ERROR")
}

func TestRegisterEndpointRejectsDuplicateEmail(t *testing.T) {
	router := newTestRouter(t)
	first := performJSONRequest(router, http.MethodPost, "/api/v1/auth/register", `{"email":"admin@example.com","password":"password123"}`, "")
	if first.Code != http.StatusCreated {
		t.Fatalf("first status = %d, body = %s", first.Code, first.Body.String())
	}
	second := performJSONRequest(router, http.MethodPost, "/api/v1/auth/register", `{"email":"admin@example.com","password":"password123"}`, "")

	if second.Code != http.StatusConflict {
		t.Fatalf("status = %d, body = %s", second.Code, second.Body.String())
	}
	assertEnvelopeCode(t, second, "CONFLICT")
}

func TestLoginEndpointRejectsInvalidCredentials(t *testing.T) {
	router := newTestRouter(t)
	response := performJSONRequest(router, http.MethodPost, "/api/v1/auth/login", `{"email":"missing@example.com","password":"password123"}`, "")

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	assertEnvelopeCode(t, response, "UNAUTHORIZED")
}

func TestRefreshEndpointReturnsOnlyTokenPair(t *testing.T) {
	router := newTestRouter(t)
	registered := performJSONRequest(router, http.MethodPost, "/api/v1/auth/register", `{"email":"admin@example.com","password":"password123"}`, "")
	var registerBody struct {
		Data struct {
			RefreshToken string `json:"refresh_token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(registered.Body.Bytes(), &registerBody); err != nil {
		t.Fatalf("decode register response: %v", err)
	}
	response := performJSONRequest(router, http.MethodPost, "/api/v1/auth/refresh", `{"refresh_token":"`+registerBody.Data.RefreshToken+`"}`, "")

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var body struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode refresh response: %v", err)
	}
	if _, ok := body.Data["access_token"]; !ok {
		t.Fatalf("response missing access_token: %s", response.Body.String())
	}
	if _, ok := body.Data["refresh_token"]; !ok {
		t.Fatalf("response missing refresh_token: %s", response.Body.String())
	}
	if _, ok := body.Data["user"]; ok {
		t.Fatalf("refresh response contains user: %s", response.Body.String())
	}
}

func TestMeEndpointRequiresAuthentication(t *testing.T) {
	router := newTestRouter(t)
	response := performJSONRequest(router, http.MethodGet, "/api/v1/me", "", "")

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestMeEndpointReturnsAuthenticatedUser(t *testing.T) {
	router := newTestRouter(t)
	registered := performJSONRequest(router, http.MethodPost, "/api/v1/auth/register", `{"email":"admin@example.com","password":"password123"}`, "")
	var registerBody struct {
		Data struct {
			AccessToken string `json:"access_token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(registered.Body.Bytes(), &registerBody); err != nil {
		t.Fatalf("decode register response: %v", err)
	}
	response := performJSONRequest(router, http.MethodGet, "/api/v1/me", "", registerBody.Data.AccessToken)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if !bytes.Contains(response.Body.Bytes(), []byte("admin@example.com")) {
		t.Fatalf("response missing user: %s", response.Body.String())
	}
}

func TestWriteErrorMapsTypedApplicationConflict(t *testing.T) {
	gin.SetMode(gin.TestMode)
	responseRecorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(responseRecorder)
	context.Set(middleware.RequestIDKey, "req_test")

	writeError(context, apperrors.New(apperrors.CodeConflict, "resource conflict", nil))

	if responseRecorder.Code != http.StatusConflict {
		t.Fatalf("status = %d, body = %s", responseRecorder.Code, responseRecorder.Body.String())
	}
	assertEnvelopeCode(t, responseRecorder, "CONFLICT")
}

func newTestRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	manager := security.NewJWTManager("test-secret-with-enough-length", time.Minute, time.Hour)
	usecase := bizidentity.NewUsecase(
		memory.NewUserRepository(),
		memory.NewTokenStore(),
		security.BcryptPasswordHasher{Cost: 4},
		identitydata.NewTokenManager(manager),
		idgen.New,
		nil,
	)
	handler := NewHandler(usecase, middleware.Auth(manager))
	router := gin.New()
	router.Use(middleware.RequestID())
	handler.RegisterRoutes(router.Group("/api/v1"))
	return router
}

func performJSONRequest(router http.Handler, method, path, body, accessToken string) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	if accessToken != "" {
		request.Header.Set("Authorization", "Bearer "+accessToken)
	}
	router.ServeHTTP(response, request)
	return response
}

func assertEnvelopeCode(t *testing.T, response *httptest.ResponseRecorder, want string) {
	t.Helper()
	var body struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != want {
		t.Fatalf("code = %q, want %q; body = %s", body.Code, want, response.Body.String())
	}
}
