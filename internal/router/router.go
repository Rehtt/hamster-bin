package router

import (
	"io/fs"
	"net/http"
	"strings"

	hamsterbin "github.com/Rehtt/hamster-bin"
	"github.com/Rehtt/hamster-bin/internal/config"
	"github.com/Rehtt/hamster-bin/internal/handlers"
	"github.com/Rehtt/hamster-bin/internal/middleware"
	"github.com/Rehtt/hamster-bin/internal/parser"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var allowedOrigins = map[string]struct{}{
	"http://localhost:5173": {},
	"http://localhost:8080": {},
}

// Setup 设置路由
func Setup(db *gorm.DB, parserManager *parser.ParserManager, cfg *config.Config) *gin.Engine {
	// 设置为发布模式（生产环境）
	// gin.SetMode(gin.ReleaseMode)

	r := gin.Default()

	// CORS 中间件
	r.Use(corsMiddleware())

	// 初始化 Handlers
	categoryHandler := handlers.NewCategoryHandler(db)
	supplierHandler := handlers.NewSupplierHandler(db)
	componentHandler := handlers.NewComponentHandler(db)
	stockLogHandler := handlers.NewStockLogHandler(db)
	statsHandler := handlers.NewStatsHandler(db)
	parserHandler := handlers.NewParserHandler(parserManager)
	authHandler := handlers.NewAuthHandler(cfg)
	authMiddleware := middleware.AuthMiddleware(cfg)

	// API 路由组
	v1 := r.Group("/api/v1")
	{
		authGroup := v1.Group("/auth")
		{
			authGroup.POST("/login", authHandler.Login)
			authGroup.POST("/logout", authHandler.Logout)
			authGroup.GET("/me", authHandler.Me)
		}

		protected := v1.Group("")
		protected.Use(authMiddleware)
		{
			// 分类管理
			categories := protected.Group("/categories")
			{
				categories.GET("", categoryHandler.GetAll)
				categories.GET("/:id", categoryHandler.GetByID)
				categories.POST("", categoryHandler.Create)
				categories.PUT("/:id", categoryHandler.Update)
				categories.DELETE("/:id", categoryHandler.Delete)
			}

			// 供应商管理
			suppliers := protected.Group("/suppliers")
			{
				suppliers.GET("", supplierHandler.GetAll)
				suppliers.GET("/:id", supplierHandler.GetByID)
				suppliers.POST("", supplierHandler.Create)
			}

			// 元件管理
			components := protected.Group("/components")
			{
				components.GET("", componentHandler.GetAll)
				components.GET("/options", componentHandler.GetOptions)
				components.GET("/export", componentHandler.ExportCSV)
				components.PATCH("/batch-location", componentHandler.BatchUpdateLocation)
				components.PATCH("/generate-numbers", componentHandler.GenerateMissingNumbers)
				components.GET("/:id", componentHandler.GetByID)
				components.POST("", componentHandler.Create)
				components.PUT("/:id", componentHandler.Update)
				components.DELETE("/:id", componentHandler.Delete)

				// 库存操作
				components.POST("/:id/stock", componentHandler.UpdateStock)
				components.POST("/:id/backfill-price", componentHandler.BackfillPrice)
				components.GET("/:id/logs", componentHandler.GetStockLogs)

				// 图片处理
				components.POST("/:id/image", componentHandler.UploadImage)
				components.GET("/:id/image", componentHandler.GetImage)

				// 平台解析
				components.POST("/parse", parserHandler.ParseComponent)
				components.POST("/parse-qrcode", parserHandler.ParseQRCode)
			}

			// 库存记录
			stockLogs := protected.Group("/stock-logs")
			{
				stockLogs.GET("", stockLogHandler.GetAll)
				stockLogs.POST("/:id/revoke", stockLogHandler.Revoke)
			}

			protected.GET("/stats", statsHandler.GetDashboard)

			// 平台支持
			protected.GET("/platforms", parserHandler.GetSupportedPlatforms)
		}
	}

	// 静态文件服务（前端页面）
	assets, _ := fs.Sub(hamsterbin.WebFS, "web/dist/assets")
	r.StaticFS("/assets", gin.OnlyFilesFS{FileSystem: http.FS(assets)})
	// http.FileSystem的一个坑,http.FileServer会自动处理目录下的index.html文件,但如果显式指定index.html就会无限301到./
	r.StaticFileFS("/", "web/dist/", http.FS(hamsterbin.WebFS))
	r.StaticFileFS("/logo.svg", "web/dist/logo.svg", http.FS(hamsterbin.WebFS))

	r.NoRoute(func(c *gin.Context) {
		// 如果是API请求返回404，否则返回前端页面（SPA路由支持）
		if len(c.Request.URL.Path) >= 4 && c.Request.URL.Path[:4] == "/api" {
			c.JSON(404, gin.H{"error": "路由不存在"})
		} else {
			c.Redirect(http.StatusMovedPermanently, "/")
		}
	})

	return r
}

// corsMiddleware CORS 跨域中间件
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" {
			if _, ok := allowedOrigins[origin]; ok {
				c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
				c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
			} else if strings.HasPrefix(origin, "http://127.0.0.1:") || strings.HasPrefix(origin, "http://localhost:") {
				c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
				c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
			}
		}

		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}
