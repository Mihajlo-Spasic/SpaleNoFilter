package main

import (
	"fmt"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/instagram-clone/social-service/internal/config"
	"github.com/instagram-clone/social-service/internal/controller"
	"github.com/instagram-clone/social-service/internal/middleware"
	"github.com/instagram-clone/social-service/internal/repository"
	"github.com/instagram-clone/social-service/internal/service"
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

	followRepo := repository.NewFollowRepository(db)
	blockRepo := repository.NewBlockRepository(db)
	svc := service.NewSocialService(followRepo, blockRepo, cfg)
	ctrl := controller.NewSocialController(svc)

	r := gin.Default()
	r.Use(middleware.Logger(), middleware.CORS())

	api := r.Group("/api/v1/social")
	api.Use(middleware.Auth(cfg))
	{
		// Follow
		api.POST("/follow", ctrl.Follow)
		api.DELETE("/follow/:target_id", ctrl.Unfollow)
		api.DELETE("/followers/:follower_id", ctrl.RemoveFollower)

		// Follow requests (private profiles)
		api.GET("/follow-requests/pending", ctrl.GetPendingRequests)
		api.POST("/follow-requests/respond", ctrl.RespondToRequest)

		// Lists
		api.GET("/users/:user_id/followers", ctrl.GetFollowers)
		api.GET("/users/:user_id/following", ctrl.GetFollowing)

		// Block
		api.POST("/block", ctrl.Block)
		api.DELETE("/block/:target_id", ctrl.Unblock)
		api.GET("/blocked", ctrl.GetBlocked)

		// Status
		api.GET("/status/:target_id", ctrl.GetStatus)
	}

	// Internal routes - only for other microservices
	internal := r.Group("/internal/social")
	internal.Use(middleware.InternalOnly(cfg.InternalSecret))
	{
		internal.POST("/can-view", ctrl.CanView)
		internal.GET("/following-ids/:user_id", ctrl.GetFollowingIDs)
		internal.POST("/is-blocked", ctrl.IsBlocked)
	}

	r.GET("/api/v1/social/health", ctrl.Health)

	fmt.Printf("Social service on :%s\n", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatal(err)
	}
}
