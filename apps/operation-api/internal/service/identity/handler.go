package identityservice

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"

	bizidentity "github.com/vort-ads/vort-ads-template/apps/control-api/internal/biz/identity"
	"github.com/vort-ads/vort-ads-template/apps/internal/middleware"
	"github.com/vort-ads/vort-ads-template/apps/internal/platform/apperrors"
	"github.com/vort-ads/vort-ads-template/apps/internal/platform/response"
)

type Handler struct {
	usecase        *bizidentity.Usecase
	authMiddleware []gin.HandlerFunc
}

func NewHandler(usecase *bizidentity.Usecase, auth ...gin.HandlerFunc) Handler {
	return Handler{usecase: usecase, authMiddleware: auth}
}

func (h Handler) RegisterRoutes(group *gin.RouterGroup, authenticatedMiddleware ...gin.HandlerFunc) {
	group.POST("/auth/register", h.register)
	group.POST("/auth/login", h.login)
	group.POST("/auth/refresh", h.refresh)

	meHandlers := append([]gin.HandlerFunc(nil), h.authMiddleware...)
	meHandlers = append(meHandlers, authenticatedMiddleware...)
	meHandlers = append(meHandlers, h.me)
	group.GET("/me", meHandlers...)
}

func (h Handler) register(c *gin.Context) {
	var request RegisterRequest
	if err := bindStrictJSON(c, &request); err != nil {
		writeValidationError(c)
		return
	}

	output, err := h.usecase.Register(c.Request.Context(), bizidentity.RegisterInput{
		Email: request.Email, Password: request.Password,
	})
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusCreated, response.OK(middleware.GetRequestID(c), toAuthData(output)))
}

func (h Handler) login(c *gin.Context) {
	var request LoginRequest
	if err := bindStrictJSON(c, &request); err != nil {
		writeValidationError(c)
		return
	}

	output, err := h.usecase.Login(c.Request.Context(), bizidentity.LoginInput{
		Email: request.Email, Password: request.Password,
	})
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.OK(middleware.GetRequestID(c), toAuthData(output)))
}

func (h Handler) refresh(c *gin.Context) {
	var request RefreshRequest
	if err := bindStrictJSON(c, &request); err != nil {
		writeValidationError(c)
		return
	}

	output, err := h.usecase.Refresh(c.Request.Context(), bizidentity.RefreshInput{RefreshToken: request.RefreshToken})
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.OK(middleware.GetRequestID(c), TokenPair{
		AccessToken: output.AccessToken, RefreshToken: output.RefreshToken,
	}))
}

func bindStrictJSON(c *gin.Context, destination any) error {
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return errors.New("request body must contain exactly one JSON value")
		}
		return err
	}
	return binding.Validator.ValidateStruct(destination)
}

func (h Handler) me(c *gin.Context) {
	principal, ok := middleware.GetPrincipal(c)
	if !ok || principal.UserID == "" {
		c.JSON(http.StatusUnauthorized, response.Error(
			middleware.GetRequestID(c), apperrors.CodeUnauthorized, "login required",
		))
		return
	}

	output, err := h.usecase.Me(c.Request.Context(), principal.UserID)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.OK(middleware.GetRequestID(c), toUser(output)))
}

func toAuthData(output bizidentity.AuthOutput) AuthData {
	return AuthData{
		User:         toUser(output.User),
		AccessToken:  output.AccessToken,
		RefreshToken: output.RefreshToken,
	}
}

func toUser(output bizidentity.UserOutput) User {
	return User{
		ID:        output.ID,
		Email:     output.Email,
		Roles:     append([]string(nil), output.Roles...),
		CreatedAt: output.CreatedAt,
		UpdatedAt: output.UpdatedAt,
	}
}

func writeValidationError(c *gin.Context) {
	c.JSON(http.StatusBadRequest, response.Error(
		middleware.GetRequestID(c), apperrors.CodeValidationError, "request validation failed",
	))
}

func writeError(c *gin.Context, err error) {
	status := http.StatusInternalServerError
	code := apperrors.CodeInternalError
	message := "internal server error"
	var applicationError *apperrors.Error
	if errors.As(err, &applicationError) {
		code = applicationError.Code
		message = applicationError.PublicMessage()
		switch applicationError.Code {
		case apperrors.CodeValidationError:
			status = http.StatusBadRequest
		case apperrors.CodeUnauthorized:
			status = http.StatusUnauthorized
		case apperrors.CodeForbidden:
			status = http.StatusForbidden
		case apperrors.CodeNotFound:
			status = http.StatusNotFound
		case apperrors.CodeConflict:
			status = http.StatusConflict
		case apperrors.CodeRateLimited:
			status = http.StatusTooManyRequests
		case apperrors.CodeDependencyError:
			status = http.StatusServiceUnavailable
		default:
			status = http.StatusInternalServerError
			code = apperrors.CodeInternalError
			message = "internal server error"
		}
		c.JSON(status, response.Error(middleware.GetRequestID(c), code, message))
		return
	}

	switch {
	case errors.Is(err, bizidentity.ErrInvalidEmail), errors.Is(err, bizidentity.ErrPasswordHashRequired):
		status, code, message = http.StatusBadRequest, apperrors.CodeValidationError, "request validation failed"
	case errors.Is(err, bizidentity.ErrInvalidCredentials):
		status, code, message = http.StatusUnauthorized, apperrors.CodeUnauthorized, "invalid credentials"
	case errors.Is(err, bizidentity.ErrEmailTaken):
		status, code, message = http.StatusConflict, apperrors.CodeConflict, "email already registered"
	case errors.Is(err, bizidentity.ErrUserNotFound):
		status, code, message = http.StatusNotFound, apperrors.CodeNotFound, "user not found"
	}

	c.JSON(status, response.Error(middleware.GetRequestID(c), code, message))
}
