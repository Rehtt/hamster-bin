package parser

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
)

// 只解码详情本身，不读取替代商品或推荐商品中的资料。
type lcscProductDetail struct {
	ProductCode string   `json:"productCode"`
	ProductName string   `json:"productNameEn"`
	Title       string   `json:"title"`
	Model       string   `json:"productModel"`
	Brand       string   `json:"brandNameEn"`
	Package     string   `json:"encapStandard"`
	Category    string   `json:"catalogName"`
	Intro       string   `json:"productIntroEn"`
	Description string   `json:"productDescEn"`
	Images      []string `json:"productImages"`
	PDF         string   `json:"pdfUrl"`
	PDFLink     string   `json:"pdfLinkUrl"`
	Stock       *int64   `json:"stockNumber"`
	Currency    string   `json:"currencyType"`
	Parameters  []struct {
		Name       string `json:"paramNameEn"`
		Value      string `json:"paramValueEn"`
		LocalName  string `json:"paramName"`
		LocalValue string `json:"paramValue"`
	} `json:"paramVOList"`
	Prices []struct {
		Quantity      int64           `json:"ladder"`
		ProductPrice  json.RawMessage `json:"productPrice"`
		CurrencyPrice json.RawMessage `json:"currencyPrice"`
		USDPrice      json.RawMessage `json:"usdPrice"`
	} `json:"productPriceList"`
}

func (p *LCSCParser) fetchProductDetail(ctx context.Context, code string) (*ComponentInfo, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://wmsc.lcsc.com/ftps/wm/product/detail?productCode="+url.QueryEscape(code), nil)
	if err != nil {
		return nil, "", fmt.Errorf("创建详情请求失败: %w: %w", err, ErrParserUpstream)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Origin", "https://www.lcsc.com")
	req.Header.Set("Referer", "https://www.lcsc.com/")
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("请求详情 API 失败: %w: %w", err, ErrParserUpstream)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("详情 HTTP 状态码: %d: %w", resp.StatusCode, ErrParserUpstream)
	}
	body, err := readLCSCResponse(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("读取详情失败: %w: %w", err, ErrParserUpstream)
	}
	var response struct {
		Code   int                `json:"code"`
		Result *lcscProductDetail `json:"result"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, "", fmt.Errorf("详情 JSON 无效: %w: %w", err, ErrParserContent)
	}
	if response.Code != 200 || response.Result == nil {
		return nil, "", fmt.Errorf("详情 API 未成功或无结果 (code=%d): %w", response.Code, ErrParserContent)
	}
	detail := response.Result
	if detail.ProductCode != code {
		return nil, "", fmt.Errorf("详情料号不匹配: %w", ErrParserContent)
	}
	name := firstLCSCText(detail.ProductName, detail.Title, detail.Model)
	if name == "" {
		return nil, "", fmt.Errorf("详情缺少商品名称和型号: %w", ErrParserContent)
	}
	var values []string
	for _, param := range detail.Parameters {
		key, value := firstLCSCText(param.Name, param.LocalName), firstLCSCText(param.Value, param.LocalValue)
		if key != "" && value != "" {
			values = append(values, key+": "+value)
		}
	}
	quote := &SupplierQuote{SourceName: "LCSC", SourceURL: req.URL.String(), StockQuantity: detail.Stock, PriceTiers: []SupplierPriceTier{}, QueriedAt: time.Now().UTC().Format(time.RFC3339)}
	if quote.StockQuantity != nil && *quote.StockQuantity < 0 {
		quote.StockQuantity = nil
	}
	currency := strings.ToUpper(strings.TrimSpace(detail.Currency))
	if currency != "" {
		quote.Currency = &currency
	}
	for _, tier := range detail.Prices {
		// productPrice 是接口的 USD 原始十进制串；其它币种使用 currencyPrice。
		price := lcscDecimal(tier.CurrencyPrice)
		if currency == "USD" {
			price = firstLCSCText(lcscDecimal(tier.ProductPrice), price, lcscDecimal(tier.USDPrice))
		}
		if tier.Quantity > 0 && price != "" {
			quote.PriceTiers = append(quote.PriceTiers, SupplierPriceTier{MinQuantity: tier.Quantity, UnitPrice: price})
		}
	}
	sort.SliceStable(quote.PriceTiers, func(i, j int) bool { return quote.PriceTiers[i].MinQuantity < quote.PriceTiers[j].MinQuantity })
	info := &ComponentInfo{
		Name: name, Model: detail.Model, Manufacturer: detail.Brand, Package: detail.Package,
		CategoryName: detail.Category, Description: firstLCSCText(detail.Intro, detail.Description), Value: strings.Join(values, "; "),
		DatasheetURL: firstLCSCURL(detail.PDF, detail.PDFLink), ImageURL: firstLCSCURL(detail.Images...),
		PlatformCode: code, PlatformName: "立创商城", PlatformURL: "https://www.lcsc.com/product-detail/" + code + ".html", SupplierQuote: quote,
	}
	parameters, _ := json.Marshal(detail.Parameters)
	return info, string(parameters), nil
}

func readLCSCResponse(body io.Reader) ([]byte, error) {
	const limit = 2 << 20
	data, err := io.ReadAll(io.LimitReader(body, limit+1))
	if err != nil {
		return nil, err
	}
	if len(data) > limit {
		return nil, fmt.Errorf("响应超过 2 MiB")
	}
	return data, nil
}

func firstLCSCText(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func firstLCSCURL(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if strings.HasPrefix(value, "//") {
			value = "https:" + value
		}
		u, err := url.Parse(value)
		if err == nil && (u.Scheme == "https" || u.Scheme == "http") && u.Host != "" {
			return value
		}
	}
	return ""
}

var lcscDecimalPattern = regexp.MustCompile(`^[0-9]+(?:\.[0-9]+)?$`)

func lcscDecimal(raw json.RawMessage) string {
	value := strings.TrimSpace(string(raw))
	if strings.HasPrefix(value, `"`) {
		if json.Unmarshal(raw, &value) != nil {
			return ""
		}
	}
	if !lcscDecimalPattern.MatchString(value) {
		return ""
	}
	return value
}
