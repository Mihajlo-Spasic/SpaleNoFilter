package main

import (
	"fmt"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/instagram-clone/interaction-service/internal/config"
	"github.com/instagram-clone/interaction-service/internal/controller"
	"github.com/instagram-clone/interaction-service/internal/middleware"
	"github.com/instagram-clone/interaction-service/internal/repository"
	"github.com/instagram-clone/interaction-service/internal/service"
)

func main() {
	cfg := config.Load()

	db, err := config.NewMySQLConnection(cfg)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer db.Close()

	if err := config.RunMigrations(db); err != nil {
		log.Fatalf("migrations: %v", err)
	}

	likeRepo := repository.NewLikeRepository(db)
	commentRepo := repository.NewCommentRepository(db)
	svc := service.NewInteractionService(likeRepo, commentRepo, cfg)
	ctrl := controller.NewInteractionController(svc)

	r := gin.Default()
	r.Use(middleware.Logger(), middleware.CORS())

	api := r.Group("/api/v1")
	api.Use(middleware.Auth(cfg))
	{
		// Likes
		api.POST("/posts/:post_id/like", ctrl.Like)
		api.DELETE("/posts/:post_id/like", ctrl.Unlike)
		api.GET("/posts/:post_id/liked", ctrl.HasLiked)

		// Comments
		api.POST("/posts/:post_id/comments", ctrl.AddComment)
		api.GET("/posts/:post_id/comments", ctrl.ListComments)
		api.PATCH("/comments/:comment_id", ctrl.UpdateComment)
		api.DELETE("/comments/:comment_id", ctrl.DeleteComment)
	}

	r.GET("/api/v1/interactions/health", ctrl.Health)

	fmt.Printf("Interaction service on :%s\n", cfg.Port)
	log.Fatal(r.Run(":" + cfg.Port))
}
