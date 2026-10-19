package parser

import (
	"context"
	"errors"
	"github.com/Rehtt/hamster-bin/internal/normalization"
	"strings"
	"sync"
)

// ComponentInfo 从平台解析出的元件信息
type ComponentInfo struct {
	CategoryID    uint                    `json:"category_id,omitempty"`
	Normalization *normalization.Metadata `json:"normalization,omitempty"`
	sources       []normalization.Source
	primarySource string
	semantic      bool
	SupplierQuote *SupplierQuote `json:"supplier_quote,omitempty"` // 供应商参考快照，不参与库存和采购金额计算
	Name          string         `json:"name"`                     // 元件名称
	CategoryName  string         `json:"category_name"`            // 建议分类名称
	Model         string         `json:"model"`                    // 型号
	Value         string         `json:"value"`                    // 参数值
	Package       string         `json:"package"`                  // 封装
	Description   string         `json:"description"`              // 描述
	Manufacturer  string         `json:"manufacturer"`             // 制造商
	Price         float64        `json:"price"`                    // 单价
	ImageURL      string         `json:"image_url"`                // 图片URL
	DatasheetURL  string         `json:"datasheet_url"`            // 数据手册URL
	PlatformCode  string         `json:"platform_code"`            // 平台编码
	PlatformName  string         `json:"platform_name"`            // 平台名称
	PlatformURL   string         `json:"platform_url"`             // 平台链接
}

type SupplierQuote struct {
	StockQuantity *int64              `json:"stock_quantity"`
	Currency      *string             `json:"currency"`
	PriceTiers    []SupplierPriceTier `json:"price_tiers"`
	SourceName    string              `json:"source_name,omitempty"`
	SourceURL     string              `json:"source_url,omitempty"`
	QueriedAt     string              `json:"queried_at"`
}

type SupplierPriceTier struct {
	MinQuantity int64  `json:"min_quantity"`
	UnitPrice   string `json:"unit_price"`
}

// Parser 平台解析器接口
type Parser interface {
	// GetName 获取解析器名称
	GetName() string

	// CanParse 判断是否能解析该编码
	CanParse(code string) bool

	// Parse 解析平台编码，返回元件信息
	Parse(code string) (*ComponentInfo, error)
}

// ParseOptions 解析选项
type ParseOptions struct {
	ParseMode string          `json:"parse_mode,omitempty"`
	Trace     *ParseTrace     `json:"-"`
	UseLLM    bool            `json:"use_llm"`
	Context   context.Context `json:"-"`
}

// OptionParser 支持可选解析策略的平台解析器
type OptionParser interface {
	ParseWithOptions(code string, options ParseOptions) (*ComponentInfo, error)
}

// ParserManager 解析器管理器
type ParserManager struct {
	standardsMu sync.RWMutex
	standards   normalization.Reader
	parsers     []Parser
}

var (
	// ErrNoParsersAvailable 没有可用的解析器
	ErrNoParsersAvailable = errors.New("没有可用的解析器")

	// ErrNoParserMatched 没有匹配的解析器
	ErrNoParserMatched = errors.New("无法识别的平台编码格式")

	// ErrParseFailed 解析失败
	ErrParseFailed = errors.New("解析平台数据失败")

	// ErrParserUpstream 上游平台请求失败（网络、超时、非 200 等）
	ErrParserUpstream = errors.New("上游平台请求失败")

	// ErrParserContent 页面已获取但内容无法解析（结构变化、信息不完整等）
	ErrParserContent = errors.New("页面内容解析失败")
)

// NewParserManager 创建解析器管理器
func NewParserManager() *ParserManager {
	return &ParserManager{
		parsers: make([]Parser, 0),
	}
}

// Register 注册解析器
func (m *ParserManager) Register(parser Parser) {
	m.parsers = append(m.parsers, parser)
}

// Parse 自动选择合适的解析器进行解析
func (m *ParserManager) Parse(code string) (*ComponentInfo, error) {
	return m.ParseWithOptions(code, ParseOptions{})
}

// ParseWithOptions 自动选择合适的解析器并传入解析选项
func (m *ParserManager) ParseWithOptions(code string, options ParseOptions) (*ComponentInfo, error) {
	if options.Trace != nil {
		*options.Trace = ParseTrace{Mode: options.Mode(), Calls: []ParseCall{}}
	}
	if err := options.ValidateMode(); err != nil {
		return nil, err
	}
	if len(m.parsers) == 0 {
		return nil, ErrNoParsersAvailable
	}

	s, err := m.readStandards(options.ExecutionContext())
	if err != nil {
		return nil, err
	}
	options.Context = context.WithValue(options.ExecutionContext(), standardsKey{}, s)
	for _, parser := range m.parsers {
		if parser.CanParse(code) {
			if optionParser, ok := parser.(OptionParser); ok {
				info, err := optionParser.ParseWithOptions(code, options)
				if err == nil {
					normalizeInfo(info, s)
				}
				return info, err
			}
			info, err := parser.Parse(code)
			if err == nil {
				normalizeInfo(info, s)
			}
			return info, err
		}
	}

	return nil, ErrNoParserMatched
}

// GetAvailableParsers 获取所有可用的解析器名称
func (m *ParserManager) GetAvailableParsers() []string {
	names := make([]string, 0, len(m.parsers))
	for _, parser := range m.parsers {
		names = append(names, parser.GetName())
	}
	return names
}

// Validate 在入队前验证编码和配置，不发起网络请求。
func (m *ParserManager) Validate(code string, options ParseOptions) error {
	if err := options.ValidateMode(); err != nil {
		return err
	}
	if len(m.parsers) == 0 {
		return ErrNoParsersAvailable
	}
	for _, p := range m.parsers {
		if p.CanParse(strings.TrimSpace(code)) {
			if v, ok := p.(interface{ ValidateOptions(ParseOptions) error }); ok {
				return v.ValidateOptions(options)
			}
			return nil
		}
	}
	return ErrNoParserMatched
}

func (o ParseOptions) ExecutionContext() context.Context {
	if o.Context != nil {
		return o.Context
	}
	return context.Background()
}

var ErrInvalidParseMode = errors.New("无效的 parse_mode，仅支持 standard、llm、agent")

func (o ParseOptions) Mode() string {
	if o.ParseMode != "" {
		return o.ParseMode
	}
	if o.UseLLM {
		return "llm"
	}
	return "standard"
}
func (o ParseOptions) ValidateMode() error {
	switch o.Mode() {
	case "standard", "llm", "agent":
		return nil
	}
	return ErrInvalidParseMode
}
