package parser

import (
	"fmt"
	"regexp"
	"strings"
)

// TaobaoParser 淘宝解析器（示例，展示如何扩展其他平台）
type TaobaoParser struct{}

// NewTaobaoParser 创建淘宝解析器
func NewTaobaoParser() *TaobaoParser {
	return &TaobaoParser{}
}

// GetName 获取解析器名称
func (p *TaobaoParser) GetName() string {
	return "淘宝"
}

// CanParse 判断是否为淘宝链接或商品ID
func (p *TaobaoParser) CanParse(code string) bool {
	code = strings.TrimSpace(code)
	// 淘宝商品ID通常是纯数字，12-13位
	matched, _ := regexp.MatchString(`^\d{12,13}$`, code)
	return matched
}

// Parse 解析淘宝商品ID
func (p *TaobaoParser) Parse(code string) (*ComponentInfo, error) {
	// 这里只是示例，实际需要调用淘宝 API 或爬虫
	return &ComponentInfo{
		Name:         "淘宝元件",
		Description:  fmt.Sprintf("从淘宝导入 (ID: %s)", code),
		PlatformCode: code,
		PlatformName: "淘宝",
		PlatformURL:  fmt.Sprintf("https://item.taobao.com/item.htm?id=%s", code),
	}, nil
}
