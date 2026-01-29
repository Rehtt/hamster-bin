package router

import (
	"io/fs"
	"net/http"

	hamsterbin "github.com/Rehtt/hamster-bin"
	"github.com/Rehtt/hamster-bin/internal/handlers"
	"github.com/Rehtt/hamster-bin/internal/parser"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Setup 设置路由
func Setup(db *gorm.DB, parserManager *parser.ParserManager) *gin.Engine {
	// 设置为发布模式（生产环境）
	// gin.SetMode(gin.ReleaseMode)

	r := gin.Default()

	// CORS 中间件
	r.Use(corsMiddleware())

	// 初始化 Handlers
	categoryHandler := handlers.NewCategoryHandler(db)
	componentHandler := handlers.NewComponentHandler(db)
	stockLogHandler := handlers.NewStockLogHandler(db)
	parserHandler := handlers.NewParserHandler(parserManager)

	// API 路由组
	v1 := r.Group("/api/v1")
	{
		// 分类管理
		categories := v1.Group("/categories")
		{
			categories.GET("", categoryHandler.GetAll)
			categories.GET("/:id", categoryHandler.GetByID)
			categories.POST("", categoryHandler.Create)
			categories.PUT("/:id", categoryHandler.Update)
			categories.DELETE("/:id", categoryHandler.Delete)
		}

		// 元件管理
		components := v1.Group("/components")
		{
			components.GET("", componentHandler.GetAll)
			components.GET("/:id", componentHandler.GetByID)
			components.POST("", componentHandler.Create)
			components.PUT("/:id", componentHandler.Update)
			components.DELETE("/:id", componentHandler.Delete)

			// 库存操作
			components.POST("/:id/stock", componentHandler.UpdateStock)
			components.GET("/:id/logs", componentHandler.GetStockLogs)

			// 图片处理
			components.POST("/:id/image", componentHandler.UploadImage)
			components.GET("/:id/image", componentHandler.GetImage)

			// 平台解析
			components.POST("/parse", parserHandler.ParseComponent)
			components.POST("/parse-qrcode", parserHandler.ParseQRCode)
		}

		// 库存记录
		stockLogs := v1.Group("/stock-logs")
		{
			stockLogs.GET("", stockLogHandler.GetAll)
		}

		// 平台支持
		v1.GET("/platforms", parserHandler.GetSupportedPlatforms)
	}

	// 静态文件服务（前端页面）
	assets, _ := fs.Sub(hamsterbin.WebFS, "web/dist/assets")
	r.StaticFS("/assets", gin.OnlyFilesFS{FileSystem: http.FS(assets)})
	// http.FileSystem的一个坑,http.FileServer会自动处理目录下的index.html文件,但如果显式指定index.html就会无限301到./
	r.StaticFileFS("/", "web/dist/", http.FS(hamsterbin.WebFS))
	r.StaticFileFS("/vite.svg", "web/dist/assets/vite.svg", http.FS(hamsterbin.WebFS))

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
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}
