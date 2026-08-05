package identityservice

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	playvalidator "github.com/go-playground/validator/v10"

	bizidentity "github.com/vort-ads/vort-ads-template/apps/operation-api/internal/biz/identity"
	"github.com/vort-ads/vort-ads-template/internal/middleware"
	"github.com/vort-ads/vort-ads-template/internal/platform/apperrors"
	"github.com/vort-ads/vort-ads-template/internal/platform/response"
)

// ValidationDetail carries a single field-level validation failure.
type ValidationDetail struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

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

// @Summary      Register a user
// @Description  Create a new account and return an access/refresh token pair.
// @Tags         Authentication
// @Accept       json
// @Produce      json
// @Param        request  body      RegisterRequest  true  "Registration payload"
// @Success      201      {object}  response.Envelope{data=AuthData}
// @Failure      400      {object}  response.Envelope
// @Failure      409      {object}  response.Envelope
// @Failure      429      {object}  response.Envelope
// @Failure      500      {object}  response.Envelope
// @Failure      503      {object}  response.Envelope
// @Failure      504      {object}  response.Envelope
// @Router       /auth/register [post]
func (h Handler) register(c *gin.Context) {
	var request RegisterRequest
	if err := bindStrictJSON(c, &request); err != nil {
		writeValidationError(c, err)
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

// @Summary      Authenticate a user
// @Description  Verify credentials and return an access/refresh token pair.
// @Tags         Authentication
// @Accept       json
// @Produce      json
// @Param        request  body      LoginRequest  true  "Login payload"
// @Success      200      {object}  response.Envelope{data=AuthData}
// @Failure      400      {object}  response.Envelope
// @Failure      401      {object}  response.Envelope
// @Failure      429      {object}  response.Envelope
// @Failure      500      {object}  response.Envelope
// @Failure      503      {object}  response.Envelope
// @Failure      504      {object}  response.Envelope
// @Router       /auth/login [post]
func (h Handler) login(c *gin.Context) {
	var request LoginRequest
	if err := bindStrictJSON(c, &request); err != nil {
		writeValidationError(c, err)
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

// @Summary      Rotate a refresh token
// @Description  Consume a refresh token and issue a new token pair.
// @Tags         Authentication
// @Accept       json
// @Produce      json
// @Param        request  body      RefreshRequest  true  "Refresh payload"
// @Success      200      {object}  response.Envelope{data=TokenPair}
// @Failure      400      {object}  response.Envelope
// @Failure      401      {object}  response.Envelope
// @Failure      429      {object}  response.Envelope
// @Failure      500      {object}  response.Envelope
// @Failure      503      {object}  response.Envelope
// @Failure      504      {object}  response.Envelope
// @Router       /auth/refresh [post]
func (h Handler) refresh(c *gin.Context) {
	var request RefreshRequest
	if err := bindStrictJSON(c, &request); err != nil {
		writeValidationError(c, err)
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

// @Summary      Get the authenticated user
// @Description  Return the profile of the user identified by the bearer token.
// @Tags         Users
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  response.Envelope{data=User}
// @Failure      401  {object}  response.Envelope
// @Failure      429  {object}  response.Envelope
// @Failure      500  {object}  response.Envelope
// @Failure      503  {object}  response.Envelope
// @Failure      504  {object}  response.Envelope
// @Router       /me [get]
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

func writeValidationError(c *gin.Context, err error) {
	status := http.StatusBadRequest
	message := "request validation failed"
	if errors.Is(err, middleware.ErrBodyTooLarge) {
		status = http.StatusRequestEntityTooLarge
		message = "request body too large"
	}
	envelope := response.Error(
		middleware.GetRequestID(c), apperrors.CodeValidationError, message,
	)
	envelope.Data = extractValidationDetails(err)
	c.JSON(status, envelope)
}

func extractValidationDetails(err error) []ValidationDetail {
	var validationErrors playvalidator.ValidationErrors
	if !errors.As(err, &validationErrors) {
		return nil
	}
	details := make([]ValidationDetail, 0, len(validationErrors))
	for _, fieldError := range validationErrors {
		details = append(details, ValidationDetail{
			Field:   jsonFieldName(fieldError),
			Message: validationMessage(fieldError),
		})
	}
	return details
}

func jsonFieldName(fieldError playvalidator.FieldError) string {
	namespace := fieldError.StructNamespace()
	// go-playground uses Struct.Field; extract the last segment and lower first char
	parts := strings.Split(namespace, ".")
	if len(parts) < 2 {
		return strings.ToLower(namespace)
	}
	name := parts[len(parts)-1]
	return strings.ToLower(name[:1]) + name[1:]
}

func validationMessage(fieldError playvalidator.FieldError) string {
	switch fieldError.Tag() {
	case "required":
		return "this field is required"
	case "email":
		return "must be a valid email address"
	case "min":
		return fmt.Sprintf("must be at least %s characters", fieldError.Param())
	case "max":
		return fmt.Sprintf("must be at most %s characters", fieldError.Param())
	default:
		return fmt.Sprintf("validation failed on %s", fieldError.Tag())
	}
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
		case apperrors.CodeTooLarge:
			status = http.StatusRequestEntityTooLarge
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
