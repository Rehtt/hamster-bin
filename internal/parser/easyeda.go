package parser

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

func easyEDAEndpoint(host, code string) string {
	return "https://" + host + "/api/eda/product/search?" + url.Values{"keyword": {code}, "type": {"3"}, "page": {"1"}, "pageSize": {"10"}}.Encode()
}
func (p *LCSCParser) fetchEasyEDA(ctx context.Context, code string) (*ComponentInfo, string, error) {
	var reasons []string
	for _, host := range []string{"pro.easyeda.com", "pro.lceda.cn"} {
		if err := ctx.Err(); err != nil {
			return nil, "", err
		}
		endpoint := easyEDAEndpoint(host, code)
		info, params, err := recordQuery(ctx, "easyeda_product_search", code, endpoint, false, func() (*ComponentInfo, string, error) { return p.fetchEasyEDAEndpoint(ctx, code, endpoint) })
		if err == nil {
			return info, params, nil
		}
		reasons = append(reasons, host+": "+err.Error())
	}
	return nil, "", fmt.Errorf("EasyEDA 查询失败: %s: %w", strings.Join(reasons, "；"), ErrParserContent)
}

// 只解码商品资料，device_info 的绘图与模型数据不进入解析结果或模型输入。
type easyEDAProduct struct {
	Code         string          `json:"code"`
	Number       string          `json:"number"`
	Name         string          `json:"name"`
	Model        string          `json:"model"`
	MPN          string          `json:"mpn"`
	Brand        string          `json:"brandName"`
	Manufacturer string          `json:"manufacturer"`
	Standard     string          `json:"standard"`
	Package      string          `json:"package"`
	Category     string          `json:"catalogName"`
	Description  string          `json:"desc"`
	Intro        string          `json:"intro"`
	Image        json.RawMessage `json:"image"`
	PDF          string          `json:"pdfFileUrl"`
	PDFHTML      string          `json:"pdfHtmlUrl"`
	Stock        json.RawMessage `json:"stockNumber"`
	Currency     string          `json:"currency"`
	CurrencyType string          `json:"currencyType"`
	Prices       []struct {
		Quantity json.RawMessage `json:"startNumber"`
		Price    json.RawMessage `json:"price"`
	} `json:"priceList"`
	InternationalPrices [][]json.RawMessage `json:"price"`
	Device              struct {
		Attributes  map[string]string `json:"attributes"`
		Description string            `json:"description"`
		Images      []string          `json:"images"`
		Tags        struct {
			Child struct {
				Name      string `json:"name"`
				LocalName string `json:"name_cn"`
			} `json:"child_tag"`
		} `json:"tags"`
	} `json:"device_info"`
}

func (p *LCSCParser) fetchEasyEDAEndpoint(ctx context.Context, code, endpoint string) (*ComponentInfo, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("请求失败: %w: %w", err, ErrParserUpstream)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("HTTP %d: %w", resp.StatusCode, ErrParserUpstream)
	}
	body, err := readLCSCResponse(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("读取响应: %w: %w", err, ErrParserUpstream)
	}
	var response struct {
		Code    *int  `json:"code"`
		Success *bool `json:"success"`
		OK      *bool `json:"ok"`
		Result  *struct {
			Products []easyEDAProduct `json:"productList"`
		} `json:"result"`
	}
	if err = json.Unmarshal(body, &response); err != nil {
		return nil, "", fmt.Errorf("JSON 无效: %w: %w", err, ErrParserContent)
	}
	if response.Code == nil || (*response.Code != 0 && *response.Code != 200) || response.Result == nil || response.Result.Products == nil || (response.Success != nil && !*response.Success) || (response.OK != nil && !*response.OK) {
		return nil, "", fmt.Errorf("业务状态或结果无效: %w", ErrParserContent)
	}
	for _, product := range response.Result.Products {
		if firstLCSCText(product.Code, product.Number) != code {
			continue
		}
		info, params := mapEasyEDA(product, code, endpoint)
		if info.Name != "" {
			return info, params, nil
		}
	}
	return nil, "", fmt.Errorf("无有效精确匹配 %s: %w", code, ErrParserContent)
}
func easyEDAInteger(raw json.RawMessage) *int64 {
	text := lcscDecimal(raw)
	if text == "" {
		return nil
	}
	n, err := strconv.ParseInt(text, 10, 64)
	if err != nil {
		return nil
	}
	return &n
}
func mapEasyEDA(p easyEDAProduct, code, endpoint string) (*ComponentInfo, string) {
	attrs := p.Device.Attributes
	var params []string
	// 只采用非绘图属性，稳定排序以便缓存、测试及模型阅读。
	excluded := map[string]bool{"LCSC Part Name": true, "Supplier Part": true, "Manufacturer": true, "Manufacturer Part": true, "Supplier Footprint": true, "Datasheet": true, "Supplier": true, "Add into BOM": true, "Convert to PCB": true, "Symbol": true, "Designator": true, "Footprint": true, "3D Model": true, "3D Model Title": true, "3D Model Transform": true, "Name": true}
	keys := make([]string, 0, len(attrs))
	for key := range attrs {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if !excluded[key] && strings.TrimSpace(attrs[key]) != "" {
			params = append(params, key+": "+attrs[key])
		}
	}
	var images []string
	var imageText string
	if json.Unmarshal(p.Image, &imageText) == nil {
		images = strings.Split(imageText, "<$>")
	} else {
		var list []struct {
			Large  string `json:"900x900"`
			Medium string `json:"224x224"`
		}
		if json.Unmarshal(p.Image, &list) == nil {
			for _, im := range list {
				images = append(images, im.Large, im.Medium)
			}
		}
	}
	images = append(images, p.Device.Images...)
	quote := &SupplierQuote{StockQuantity: easyEDAInteger(p.Stock), PriceTiers: []SupplierPriceTier{}, QueriedAt: time.Now().UTC().Format(time.RFC3339), SourceName: "EasyEDA", SourceURL: endpoint}
	currency := strings.ToUpper(firstLCSCText(p.Currency, p.CurrencyType))
	if currency != "" {
		quote.Currency = &currency
	}
	for _, tier := range p.Prices {
		quantity := easyEDAInteger(tier.Quantity)
		price := lcscDecimal(tier.Price)
		if quantity != nil && *quantity > 0 && price != "" {
			quote.PriceTiers = append(quote.PriceTiers, SupplierPriceTier{MinQuantity: *quantity, UnitPrice: price})
		}
	}
	if len(quote.PriceTiers) == 0 {
		for _, tier := range p.InternationalPrices {
			if len(tier) < 2 {
				continue
			}
			quantity := easyEDAInteger(tier[0])
			price := lcscDecimal(tier[1])
			if quantity != nil && *quantity > 0 && price != "" {
				quote.PriceTiers = append(quote.PriceTiers, SupplierPriceTier{MinQuantity: *quantity, UnitPrice: price})
			}
		}
	}
	sort.SliceStable(quote.PriceTiers, func(i, j int) bool { return quote.PriceTiers[i].MinQuantity < quote.PriceTiers[j].MinQuantity })
	info := &ComponentInfo{Name: firstLCSCText(p.Name, attrs["LCSC Part Name"], p.Model, p.MPN), Model: firstLCSCText(p.Model, p.MPN, attrs["Manufacturer Part"]), Manufacturer: firstLCSCText(p.Brand, p.Manufacturer, attrs["Manufacturer"]), Package: firstLCSCText(p.Standard, p.Package, attrs["Supplier Footprint"]), CategoryName: firstLCSCText(p.Category, p.Device.Tags.Child.LocalName, p.Device.Tags.Child.Name), Description: firstLCSCText(p.Description, p.Intro, p.Device.Description), Value: strings.Join(params, "; "), ImageURL: firstLCSCURL(images...), DatasheetURL: firstLCSCURL(p.PDF, p.PDFHTML, attrs["Datasheet"]), PlatformCode: code, PlatformName: "立创商城", PlatformURL: "https://www.lcsc.com/product-detail/" + code + ".html", SupplierQuote: quote}
	return info, strings.Join(params, "; ")
}
