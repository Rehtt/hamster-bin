package parser

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// LCSCParser 立创商城解析器
type LCSCParser struct {
	client *http.Client
}

// NewLCSCParser 创建立创商城解析器
func NewLCSCParser() *LCSCParser {
	return &LCSCParser{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// GetName 获取解析器名称
func (p *LCSCParser) GetName() string {
	return "立创商城(LCSC)"
}

// CanParse 判断是否为立创商城编码
// 立创商城编码格式: C + 数字，例如 C2040, C123456
func (p *LCSCParser) CanParse(code string) bool {
	// 支持格式：C123456 或 c123456
	matched, _ := regexp.MatchString(`^[Cc]\d+$`, strings.TrimSpace(code))
	return matched
}

// Parse 解析立创商城编码
func (p *LCSCParser) Parse(code string) (*ComponentInfo, error) {
	code = strings.TrimSpace(code)
	code = strings.ToUpper(code) // 统一转为大写

	return p.parseByAPI(code)
}

// parseByAPI 通过 API 解析
func (p *LCSCParser) parseByAPI(code string) (*ComponentInfo, error) {
	url := fmt.Sprintf("https://www.jlc-smt.com/lcsc/detail/%s.html", code)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP 状态码: %d", resp.StatusCode)
	}

	// 解析 HTML
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("解析HTML失败: %w", err)
	}

	// 查找 BaseInfo_component-info__yuOgz 容器
	baseInfo := doc.Find(".BaseInfo_component-info__yuOgz")
	if baseInfo.Length() == 0 {
		return nil, fmt.Errorf("未找到元件信息容器")
	}

	info := &ComponentInfo{
		PlatformCode: code,
		PlatformName: "立创商城",
		PlatformURL:  url,
	}

	// 解析产品名称
	name := baseInfo.Find("h1.BaseInfo_component-name__7OSgG").Text()
	info.Value = strings.TrimSpace(name)

	// 解析 dl 列表中的信息
	baseInfo.Find("dl > div").Each(func(i int, s *goquery.Selection) {
		dt := strings.TrimSpace(s.Find("dt").Text())
		dd := strings.TrimSpace(s.Find("dd").Text())

		switch dt {
		case "品牌名称":
			info.Manufacturer = dd
		case "厂家型号":
			info.Model = dd
		case "商品编号":
			// 验证商品编号是否匹配
			if dd != code {
				// 可以记录警告，但不一定要中断
			}
			info.Description += fmt.Sprintf("商品编号: %s\n", dd)
		case "商品封装":
			info.Package = dd
		case "包装方式":
			// 可以存储到 Description 中
			if info.Description != "" {
				info.Description += " | "
			}
			info.Description += fmt.Sprintf("包装: %s", dd)
		}
	})

	goodsParameter := doc.Find(".GoodsParameter_table__VYg5o")
	if goodsParameter.Length() > 0 {
		goodsParameter.Find("tbody > tr").Each(func(i int, s *goquery.Selection) {
			var key, value string
			for i, v := range s.Find("td").EachIter() {
				switch i {
				case 0:
					key = strings.TrimSpace(v.Text())
				case 1:
					value = strings.TrimSpace(v.Text())
				}
			}
			switch key {
			case "商品目录":
				info.Name = value
			}
		})
	}

	h, ok := doc.Find(".DataBookPDF_link__h7pPt").Attr("href")
	if ok {
		info.DatasheetURL = h
	}

	// 如果没有解析到基本信息，返回错误
	if info.Name == "" {
		return nil, fmt.Errorf("未能解析元件名称")
	}

	return info, nil
}
