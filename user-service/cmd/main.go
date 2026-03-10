package main

import (
	"fmt"
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/instagram-clone/user-service/internal/config"
	"github.com/instagram-clone/user-service/internal/controller"
	"github.com/instagram-clone/user-service/internal/middleware"
	"github.com/instagram-clone/user-service/internal/repository"
	"github.com/instagram-clone/user-service/internal/service"
)

func main() {
	cfg := config.Load()

	db, err := config.NewMySQLConnection(cfg)
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}
	defer db.Close()

	if err := config.RunMigrations(db); err != nil {
		log.Fatalf("migrations: %v", err)
	}

	userRepo := repository.NewUserRepository(db)
	authClient := service.NewAuthClient(cfg)
	userSvc := service.NewUserService(userRepo, authClient, cfg)
	ctrl := controller.NewUserController(userSvc)

	r := gin.Default()
	r.Use(middleware.Logger(), middleware.CORS())

	api := r.Group("/api/v1")

	public := api.Group("")
	{
		public.POST("/auth/register", ctrl.Register)
		public.POST("/auth/login", ctrl.Login)
		public.POST("/auth/refresh", ctrl.RefreshToken)

		// Profile viewable without login (basic info only)
		public.GET("/users/:username/profile", ctrl.GetPublicProfile)
	}

	protected := api.Group("")
	protected.Use(middleware.Auth(cfg))
	{
		// Auth lifecycle
		protected.POST("/auth/logout", ctrl.Logout)
		protected.POST("/auth/logout-all", ctrl.LogoutAll)
		protected.PUT("/auth/change-password", ctrl.ChangePassword)

		// Own profile
		protected.GET("/users/me", ctrl.GetMyProfile)
		protected.PUT("/users/me", ctrl.UpdateProfile)
		protected.PUT("/users/me/avatar", ctrl.UpdateAvatar)
		protected.DELETE("/users/me", ctrl.DeleteAccount)

		// Discovery
		protected.GET("/users/search", ctrl.SearchUsers)
		protected.GET("/users/id/:id", ctrl.GetPublicProfileByID)
	}

	// social-service calls /internal/users/:id/is-private to decide auto-accept vs request
	// post-service, interaction-service call /internal/users/:id for ownership checks
	internal := r.Group("/internal/users")
	internal.Use(middleware.InternalOnly(cfg.InternalSecret))
	{
		internal.GET("/:id", ctrl.GetUserByID)
		internal.GET("/by-username/:username", ctrl.GetUserByUsername)
		internal.GET("/:id/is-private", ctrl.IsPrivate)
	}

	api.GET("/health", ctrl.Health)

	port := cfg.Port
	if port == "" {
		port = "8080"
	}
	fmt.Printf("User service running on :%s\n", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Server failed: %v", err)
		os.Exit(1)
	}
}
