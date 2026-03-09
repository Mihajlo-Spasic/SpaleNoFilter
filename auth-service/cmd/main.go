package main

import (
	"fmt"
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/instagram-clone/auth-service/internal/config"
	"github.com/instagram-clone/auth-service/internal/controller"
	"github.com/instagram-clone/auth-service/internal/middleware"
	"github.com/instagram-clone/auth-service/internal/repository"
	"github.com/instagram-clone/auth-service/internal/service"
)

func main() {
	cfg := config.Load()

	db, err := config.NewMySQLConnection(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	if err := config.RunMigrations(db); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// Dependency injection: Repository → Service → Controller
	tokenRepo := repository.NewTokenRepository(db)
	authService := service.NewAuthService(tokenRepo, cfg)
	authController := controller.NewAuthController(authService)

	r := gin.Default()
	r.Use(middleware.Logger())
	r.Use(middleware.CORS())

	api := r.Group("/api/v1/auth")
	{
		api.POST("/validate", authController.ValidateToken)
		api.POST("/refresh", authController.RefreshToken)
		api.POST("/revoke", authController.RevokeToken)
		api.GET("/health", authController.Health)
	}

	// Internal routes (called by user-service)
	internal := r.Group("/internal/auth")
	internal.Use(middleware.InternalOnly(cfg.InternalSecret))
	{
		internal.POST("/issue", authController.IssueToken)
		internal.POST("/invalidate-user", authController.InvalidateUserTokens)
	}

	port := cfg.Port
	if port == "" {
		port = "8081"
	}

	fmt.Printf("Auth service running on :%s\n", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Server failed: %v", err)
		os.Exit(1)
	}
}
