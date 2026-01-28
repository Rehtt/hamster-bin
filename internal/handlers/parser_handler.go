package handlers

import (
	"net/http"

	"github.com/Rehtt/hamster-bin/internal/parser"
	"github.com/gin-gonic/gin"
)

type ParserHandler struct {
	manager *parser.ParserManager
}

func NewParserHandler(manager *parser.ParserManager) *ParserHandler {
	return &ParserHandler{
		manager: manager,
	}
}

// ParseRequest 解析请求
type ParseRequest struct {
	Code string `json:"code" binding:"required"` // 平台编码
}

// QRCodeParseRequest 二维码解析请求
type QRCodeParseRequest struct {
	QRCodeData string `json:"qrcode_data" binding:"required"` // 二维码原始数据
}

// ParseComponent 解析平台编码，返回元件信息
// @route POST /api/v1/components/parse
func (h *ParserHandler) ParseComponent(c *gin.Context) {
	var req ParseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供平台编码"})
		return
	}

	// 调用解析器
	info, err := h.manager.Parse(req.Code)
	if err != nil {
		switch err {
		case parser.ErrNoParsersAvailable:
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "解析服务不可用"})
		case parser.ErrNoParserMatched:
			c.JSON(http.StatusBadRequest, gin.H{"error": "无法识别的平台编码格式，请检查编码是否正确"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "解析失败: " + err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": info,
		"message": "解析成功",
	})
}

// GetSupportedPlatforms 获取支持的平台列表
// @route GET /api/v1/platforms
func (h *ParserHandler) GetSupportedPlatforms(c *gin.Context) {
	platforms := h.manager.GetAvailableParsers()
	c.JSON(http.StatusOK, gin.H{
		"data": platforms,
		"count": len(platforms),
	})
}

// ParseQRCode 解析二维码，返回元件信息和数量
// @route POST /api/v1/components/parse-qrcode
func (h *ParserHandler) ParseQRCode(c *gin.Context) {
	var req QRCodeParseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供二维码数据"})
		return
	}

	// 解析二维码内容
	qrData, err := parser.ParseQRCode(req.QRCodeData)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "解析二维码失败: " + err.Error()})
		return
	}

	// 使用提取的编码调用解析器获取元件信息
	info, err := h.manager.Parse(qrData.Code)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "解析元件编码失败: " + err.Error(),
			"qrcode_data": qrData,
		})
		return
	}

	// 返回元件信息和数量
	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"component": info,
			"quantity": qrData.Quantity,
			"qrcode_info": qrData,
		},
		"message": "解析成功",
	})
}
