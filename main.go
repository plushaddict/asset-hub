package main

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"

	"assethub/internal/config"
	"assethub/internal/handler"
	"assethub/internal/repo"
	"assethub/internal/service"
)

//go:embed static
var staticFS embed.FS

func main() {
	cfg, err := config.Load("config.yaml")
	if err != nil {
		log.Fatalf("读取配置失败（请确认当前目录有 config.yaml）：%v", err)
	}

	db, err := repo.Open(cfg.Database.Path)
	if err != nil {
		log.Fatalf("打开数据库失败：%v", err)
	}
	defer db.Close()
	if err = repo.Migrate(db); err != nil {
		log.Fatalf("初始化表结构失败：%v", err)
	}

	settingsRepo := repo.NewSettingsRepo(db)
	categoryRepo := repo.NewCategoryRepo(db)
	tagRepo := repo.NewTagRepo(db)
	// OSS / 业务 token 仅来自库与设置页；未配置时 ready=false，不退出。
	settingsSvc := service.NewSettingsService(settingsRepo, categoryRepo)
	assetSvc := service.NewAssetService(
		repo.NewAssetRepo(db), repo.NewAssetFileRepo(db),
		categoryRepo, tagRepo, settingsSvc,
	)
	catSvc := service.NewCategoryService(categoryRepo)
	tagSvc := service.NewTagService(tagRepo)

	pages, err := loadPages()
	if err != nil {
		log.Fatalf("加载页面失败：%v", err)
	}
	h := handler.New(assetSvc, settingsSvc, catSvc, tagSvc, cfg, pages)

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())
	r.MaxMultipartMemory = service.MaxUploadSize

	store := cookie.NewStore([]byte(cfg.Server.Secret))
	store.Options(sessions.Options{
		Path:     "/",
		MaxAge:   cfg.Server.SessionMaxAge,
		HttpOnly: true,
	})
	r.Use(sessions.Sessions("assethub", store))

	// 静态资源 + 页面
	r.StaticFS("/static", http.FS(pages))
	r.GET("/login", h.LoginPage)
	r.GET("/", h.AuthPage(), h.IndexPage)
	r.GET("/asset/:id", h.AuthPage(), h.DetailPage)
	r.GET("/settings", h.AuthPage(), h.SettingsPage)

	// 登录
	r.POST("/api/login", h.Login)
	r.POST("/api/logout", h.Logout)

	// 业务查询接口（无需登录，可配 token）
	r.GET("/api/business/asset", h.BusinessAsset)

	// 管理接口
	api := r.Group("/api", h.AuthAPI())
	{
		api.GET("/settings", h.GetSettings)
		api.PUT("/settings", h.PutSettings)
		api.GET("/setup/status", h.SetupStatus)

		api.GET("/categories", h.ListCategories)
		api.POST("/categories", h.CreateCategory)
		api.PUT("/categories/:id", h.UpdateCategory)
		api.DELETE("/categories/:id", h.DeleteCategory)

		api.GET("/tags", h.ListTags)
		api.POST("/tags", h.CreateTag)
		api.PUT("/tags/:id", h.UpdateTag)
		api.DELETE("/tags/:id", h.DeleteTag)

		api.POST("/asset", h.CreateAsset)
		api.GET("/asset/list", h.AssetList)
		api.GET("/asset/:id", h.AssetDetail)
		api.PUT("/asset/:id", h.UpdateAsset)
		api.DELETE("/asset/:id", h.DeleteAsset)

		api.POST("/asset/:id/files", h.UploadFile)
		api.PUT("/asset/:id/files/:fid", h.UpdateFileRemark)
		api.PUT("/asset/:id/files/:fid/primary", h.SetPrimary)
		api.DELETE("/asset/:id/files/:fid", h.DeleteFile)
	}

	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	log.Printf("AssetHub 启动：http://127.0.0.1%s  (ossReady=%v)",
		addr, settingsSvc.SetupStatus().OssReady)
	if err = r.Run(addr); err != nil {
		log.Fatal(err)
	}
}

// loadPages 优先读工作目录下的 static/（改 HTML 后刷新即可）；
// 若目录不存在（例如只发布单二进制），则回退到 go:embed。
// 强制内嵌：ASSETHUB_EMBED_STATIC=1
func loadPages() (fs.FS, error) {
	if os.Getenv("ASSETHUB_EMBED_STATIC") == "1" {
		log.Printf("页面资源：go:embed static（ASSETHUB_EMBED_STATIC=1）")
		return fs.Sub(staticFS, "static")
	}
	if info, err := os.Stat("static"); err == nil && info.IsDir() {
		log.Printf("页面资源：本地目录 static/（改 HTML 后刷新即可，无需重启）")
		return os.DirFS("static"), nil
	}
	log.Printf("页面资源：go:embed static")
	return fs.Sub(staticFS, "static")
}
