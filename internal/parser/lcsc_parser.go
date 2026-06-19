package parser

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/Rehtt/hamster-bin/internal/llm"
)

type JSONCompleter interface {
	CompleteJSON(ctx context.Context, messages []llm.Message) (string, error)
	Configured() bool
}

// LCSCParser 立创商城解析器
type LCSCParser struct {
	client *http.Client
	llm    JSONCompleter
}

// NewLCSCParser 创建立创商城解析器
func NewLCSCParser(llmClient ...JSONCompleter) *LCSCParser {
	var completer JSONCompleter
	if len(llmClient) > 0 {
		completer = llmClient[0]
	}

	return &LCSCParser{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		llm: completer,
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
	return p.ParseWithOptions(code, ParseOptions{})
}

func (p *LCSCParser) ParseWithOptions(code string, options ParseOptions) (*ComponentInfo, error) {
	code = strings.TrimSpace(code)
	code = strings.ToUpper(code) // 统一转为大写

	return p.parseByAPI(code, options)
}

// parseByAPI 通过 API 解析
func (p *LCSCParser) parseByAPI(code string, options ...ParseOptions) (*ComponentInfo, error) {
	parseOptions := ParseOptions{}
	if len(options) > 0 {
		parseOptions = options[0]
	}
	if parseOptions.UseLLM && (p.llm == nil || !p.llm.Configured()) {
		return nil, llm.ErrNotConfigured
	}

	url := fmt.Sprintf("https://www.jlc-smt.com/lcsc/detail/%s.html", code)
	doc, pageText, err := p.fetchDetailDocument(url)
	if err != nil {
		return nil, err
	}

	info, err := parseLCSCDetailDocument(doc, code, url)
	if parseOptions.UseLLM {
		if info == nil {
			info = &ComponentInfo{
				PlatformCode: code,
				PlatformName: "立创商城",
				PlatformURL:  url,
			}
		}
		if err := p.enrichWithLLM(context.Background(), info, pageText); err != nil {
			return nil, err
		}
		if info.Name == "" {
			return nil, fmt.Errorf("LLM 未能解析元件名称: %w", ErrParserContent)
		}
		return info, nil
	}

	if err != nil {
		return nil, err
	}
	return info, nil
}

func (p *LCSCParser) fetchDetailDocument(url string) (*goquery.Document, string, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, "", fmt.Errorf("创建请求失败: %w: %w", err, ErrParserUpstream)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("请求上游平台失败: %w: %w", err, ErrParserUpstream)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("HTTP 状态码: %d: %w", resp.StatusCode, ErrParserUpstream)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, "", fmt.Errorf("读取响应失败: %w: %w", err, ErrParserUpstream)
	}

	// 解析 HTML
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(body)))
	if err != nil {
		return nil, "", fmt.Errorf("解析HTML失败: %w: %w", err, ErrParserContent)
	}

	pageText := normalizeWhitespace(doc.Text())
	if len(pageText) > 12000 {
		pageText = pageText[:12000]
	}
	return doc, pageText, nil
}

func parseLCSCDetailDocument(doc *goquery.Document, code, url string) (*ComponentInfo, error) {
	// 查找 BaseInfo_component-info__yuOgz 容器
	baseInfo := doc.Find(".BaseInfo_component-info__yuOgz")
	if baseInfo.Length() == 0 {
		return nil, fmt.Errorf("未找到元件信息容器: %w", ErrParserContent)
	}

	info := &ComponentInfo{
		PlatformCode: code,
		PlatformName: "立创商城",
		PlatformURL:  url,
	}

	// 解析产品名称
	name := baseInfo.Find("h1.BaseInfo_component-name__7OSgG").Text()
	info.Name = strings.TrimSpace(name)

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
				info.CategoryName = value
			}
		})
	}

	h, ok := doc.Find(".DataBookPDF_link__h7pPt").Attr("href")
	if ok {
		info.DatasheetURL = h
	}

	// 如果没有解析到基本信息，返回错误
	if info.Name == "" {
		return nil, fmt.Errorf("未能解析元件名称: %w", ErrParserContent)
	}

	return info, nil
}

func (p *LCSCParser) enrichWithLLM(ctx context.Context, info *ComponentInfo, pageText string) error {
	initial, _ := json.Marshal(info)
	result, err := p.llm.CompleteJSON(ctx, []llm.Message{
		{
			Role:    "system",
			Content: "你是电子元件参数解析助手。只输出 JSON object，不要输出 Markdown。字段必须使用英文 snake_case。",
		},
		{
			Role: "user",
			Content: fmt.Sprintf(`请从嘉立创/LCSC 商品页面文本中解析电子元件信息。

输出 JSON object，只允许包含这些字段：
name, category_name, model, value, package, description, manufacturer, datasheet_url, image_url

要求：
- category_name 使用库存系统分类，例如 电阻、电容、MCU、连接器。
- name 商品名称。
- model 使用厂家型号。
- value 使用阻值、容值、芯片型号或关键参数。
- package 使用封装。
- description 用简短中文汇总其它重要参数。
- 无法确定的字段输出空字符串。

当前 DOM 初步解析结果：
%s

页面文本：
%s`, string(initial), pageText),
		},
	})
	if err != nil {
		return fmt.Errorf("LLM 请求失败: %w: %w", err, ErrParserUpstream)
	}

	var parsed struct {
		Name         string `json:"name"`
		CategoryName string `json:"category_name"`
		Model        string `json:"model"`
		Value        string `json:"value"`
		Package      string `json:"package"`
		Description  string `json:"description"`
		Manufacturer string `json:"manufacturer"`
		DatasheetURL string `json:"datasheet_url"`
		ImageURL     string `json:"image_url"`
	}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		return fmt.Errorf("解析 LLM JSON 失败: %w: %w", err, ErrParserContent)
	}

	applyString := func(target *string, value string) {
		value = strings.TrimSpace(value)
		if value != "" {
			*target = value
		}
	}

	applyString(&info.Name, parsed.Name)
	applyString(&info.CategoryName, parsed.CategoryName)
	applyString(&info.Model, parsed.Model)
	applyString(&info.Value, parsed.Value)
	applyString(&info.Package, parsed.Package)
	applyString(&info.Description, parsed.Description)
	applyString(&info.Manufacturer, parsed.Manufacturer)
	applyString(&info.DatasheetURL, parsed.DatasheetURL)
	applyString(&info.ImageURL, parsed.ImageURL)

	return nil
}

func normalizeWhitespace(value string) string {
	return strings.Join(strings.Fields(value), " ")
}
