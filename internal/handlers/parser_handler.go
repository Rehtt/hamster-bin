package handlers

import (
	"errors"
	"net/http"

	"github.com/Rehtt/hamster-bin/internal/llm"
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
	Code   string `json:"code" binding:"required"` // 平台编码
	UseLLM bool   `json:"use_llm"`                 // 是否使用 LLM 辅助解析
}

// QRCodeParseRequest 二维码解析请求
type QRCodeParseRequest struct {
	QRCodeData string `json:"qrcode_data" binding:"required"` // 二维码原始数据
	UseLLM     bool   `json:"use_llm"`                        // 是否使用 LLM 辅助解析
}

// parseErrorResponse 将解析错误映射为 HTTP 状态码和错误消息
func parseErrorResponse(err error) (int, string) {
	switch {
	case errors.Is(err, parser.ErrNoParsersAvailable):
		return http.StatusServiceUnavailable, "解析服务不可用"
	case errors.Is(err, parser.ErrNoParserMatched):
		return http.StatusBadRequest, "无法识别的平台编码格式，请检查编码是否正确"
	case errors.Is(err, llm.ErrNotConfigured):
		return http.StatusBadRequest, err.Error()
	case errors.Is(err, parser.ErrParserUpstream):
		return http.StatusBadGateway, "上游平台请求失败: " + err.Error()
	case errors.Is(err, parser.ErrParserContent):
		return http.StatusUnprocessableEntity, "页面内容解析失败: " + err.Error()
	default:
		return http.StatusInternalServerError, "解析失败: " + err.Error()
	}
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
	info, err := h.manager.ParseWithOptions(req.Code, parser.ParseOptions{UseLLM: req.UseLLM})
	if err != nil {
		status, message := parseErrorResponse(err)
		c.JSON(status, gin.H{"error": message})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":    info,
		"message": "解析成功",
	})
}

// GetSupportedPlatforms 获取支持的平台列表
// @route GET /api/v1/platforms
func (h *ParserHandler) GetSupportedPlatforms(c *gin.Context) {
	platforms := h.manager.GetAvailableParsers()
	c.JSON(http.StatusOK, gin.H{
		"data":  platforms,
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
	info, err := h.manager.ParseWithOptions(qrData.Code, parser.ParseOptions{UseLLM: req.UseLLM})
	if err != nil {
		status, message := parseErrorResponse(err)
		c.JSON(status, gin.H{
			"error":       message,
			"qrcode_data": qrData,
		})
		return
	}

	// 返回元件信息和数量
	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"component":   info,
			"quantity":    qrData.Quantity,
			"qrcode_info": qrData,
		},
		"message": "解析成功",
	})
}
