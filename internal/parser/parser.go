package parser

import "errors"

// ComponentInfo 从平台解析出的元件信息
type ComponentInfo struct {
	Name         string  `json:"name"`          // 元件名称
	Model        string  `json:"model"`         // 型号
	Value        string  `json:"value"`         // 参数值
	Package      string  `json:"package"`       // 封装
	Description  string  `json:"description"`   // 描述
	Manufacturer string  `json:"manufacturer"`  // 制造商
	Price        float64 `json:"price"`         // 单价
	ImageURL     string  `json:"image_url"`     // 图片URL
	DatasheetURL string  `json:"datasheet_url"` // 数据手册URL
	PlatformCode string  `json:"platform_code"` // 平台编码
	PlatformName string  `json:"platform_name"` // 平台名称
	PlatformURL  string  `json:"platform_url"`  // 平台链接
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

// ParserManager 解析器管理器
type ParserManager struct {
	parsers []Parser
}

var (
	// ErrNoParsersAvailable 没有可用的解析器
	ErrNoParsersAvailable = errors.New("没有可用的解析器")

	// ErrNoParserMatched 没有匹配的解析器
	ErrNoParserMatched = errors.New("无法识别的平台编码格式")

	// ErrParseFailed 解析失败
	ErrParseFailed = errors.New("解析平台数据失败")
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
	if len(m.parsers) == 0 {
		return nil, ErrNoParsersAvailable
	}

	for _, parser := range m.parsers {
		if parser.CanParse(code) {
			return parser.Parse(code)
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
