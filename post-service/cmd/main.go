package main

import (
	"fmt"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/instagram-clone/post-service/internal/config"
	"github.com/instagram-clone/post-service/internal/controller"
	"github.com/instagram-clone/post-service/internal/middleware"
	"github.com/instagram-clone/post-service/internal/repository"
	"github.com/instagram-clone/post-service/internal/service"
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

	storage, err := service.NewMinioStorage(cfg)
	if err != nil {
		log.Fatalf("minio: %v", err)
	}

	repo := repository.NewPostRepository(db)
	svc := service.NewPostService(repo, storage, cfg)
	ctrl := controller.NewPostController(svc)

	r := gin.Default()
	r.Use(middleware.Logger(), middleware.CORS())

	// Max upload: 20 files × 50 MB = 1 GB theoretical; gin default 32 MB
	r.MaxMultipartMemory = 1 << 30 // 1 GB

	api := r.Group("/api/v1")
	api.Use(middleware.Auth(cfg))
	{
		api.POST("/posts", ctrl.CreatePost)
		api.GET("/posts/:post_id", ctrl.GetPost)
		api.PATCH("/posts/:post_id/caption", ctrl.UpdateCaption)
		api.DELETE("/posts/:post_id/media/:media_id", ctrl.DeleteMedia)
		api.DELETE("/posts/:post_id", ctrl.DeletePost)
		api.POST("/upload/avatar", ctrl.UploadAvatar)
		api.GET("/users/:user_id/posts", ctrl.GetUserPosts)
	}

	internal := r.Group("/internal/posts")
	internal.Use(middleware.InternalOnly(cfg.InternalSecret))
	{
		internal.GET("/:post_id/owner", ctrl.GetPostOwner)
		internal.POST("/by-user-ids", ctrl.GetByUserIDs)
		internal.POST("/:post_id/like-count", ctrl.IncrementLike)
		internal.POST("/:post_id/comment-count", ctrl.IncrementComment)
	}

	r.GET("/api/v1/posts/health", ctrl.Health)

	fmt.Printf("Post service on :%s\n", cfg.Port)
	log.Fatal(r.Run(":" + cfg.Port))
}
