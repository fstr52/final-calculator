package orchestrator

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/fstr52/final-calculator/internal/db/postgresql"
	"github.com/fstr52/final-calculator/internal/logger"
	"github.com/fstr52/final-calculator/internal/token"
	"github.com/fstr52/final-calculator/internal/user"
	"github.com/fstr52/final-calculator/internal/user/db"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

type AuthHandler struct {
	logger       logger.Logger
	userStorage  user.Storage
	tokenManager token.TokenManager
}

type AuthHandlerConfig struct {
	Client     postgresql.Client
	SigningKey string
	Logger     logger.Logger
}

func NewAuthHander(config AuthHandlerConfig) (*AuthHandler, error) {
	userStorage := db.NewStorage(config.Client, config.Logger)
	tokenManager, err := token.NewManager(config.SigningKey)
	if err != nil {
		return nil, err
	}

	return &AuthHandler{
		logger:       config.Logger,
		userStorage:  userStorage,
		tokenManager: tokenManager,
	}, nil
}

func (a *AuthHandler) AuthorizationMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Missing Authorization header"})
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token == authHeader {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid Authorization format"})
			return
		}

		userID, err := a.tokenManager.Parse(token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			return
		}

		c.Set("userID", userID)
		c.Next()
	}
}

func (a *AuthHandler) RegisterHandler(c *gin.Context) {
	var regData struct {
		Login    string `json:"login"`
		Password string `json:"password"`
	}

	if err := c.ShouldBindBodyWithJSON(&regData); err != nil {
		a.logger.Error("Failed to decode request body",
			"error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	if regData.Login == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Login can't be empty"})
		return
	}

	if regData.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Password can't be empty"})
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(regData.Password), bcrypt.DefaultCost)
	if err != nil {
		a.logger.Error("Failed to hash password",
			"error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	user := user.User{
		Username:     regData.Login,
		PasswordHash: string(hashedPassword),
	}

	if _, err := a.userStorage.Create(c.Request.Context(), user); err != nil {
		a.logger.Error("REGISTRATION ERR", "error", err)
		if errors.Is(err, db.ErrUserAlreadyExists) {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"result": "You successfully registered!"})
}

func (a *AuthHandler) LoginHandler(c *gin.Context) {
	var logData struct {
		Login    string `json:"login"`
		Password string `json:"password"`
	}

	if err := c.ShouldBindJSON(&logData); err != nil {
		a.logger.Error("Failed to decode request body", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	if logData.Login == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Login can't be empty"})
		return
	}

	if logData.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Password can't be empty"})
		return
	}

	user, err := a.userStorage.FindByUsername(c.Request.Context(), logData.Login)
	if err != nil {
		a.logger.Warn("User not found", "login", logData.Login, "error", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid login or password"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(logData.Password)); err != nil {
		a.logger.Warn("Invalid password", "login", logData.Login)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid login or password"})
		return
	}

	token, err := a.tokenManager.NewToken(user.ID, 15*time.Minute)
	if err != nil {
		a.logger.Error("Failed to create token", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token": token,
	})
}
