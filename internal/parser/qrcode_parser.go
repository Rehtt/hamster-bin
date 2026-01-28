package parser

import (
	"fmt"
	"strings"

	"github.com/gogf/gf/v2/util/gconv"
)

// QRCodeData 二维码解析出的数据
type QRCodeData struct {
	Code     string `json:"code"`     // 元件编码
	Quantity int    `json:"quantity"` // 数量
	RawData  string `json:"raw_data"` // 原始二维码数据
	Platform string `json:"platform"` // 识别的平台
}

// ParseQRCode 解析二维码内容，提取元件编码和数量
func ParseQRCode(qrcodeData string) (*QRCodeData, error) {
	if qrcodeData == "" {
		return nil, fmt.Errorf("二维码数据为空")
	}

	if info := parseLCSCCode(qrcodeData); info != nil {
		return info, nil
	}

	return nil, fmt.Errorf("无法识别的二维码格式")
}

func parseLCSCCode(code string) *QRCodeData {
	m := make(map[string]string)
	c := strings.TrimPrefix(code, "{")
	c = strings.TrimSuffix(c, "}")
	for v := range strings.SplitSeq(c, ",") {
		kv := strings.Split(v, ":")
		if len(kv) == 2 {
			m[kv[0]] = kv[1]
		}
	}
	var qr struct {
		Code     string `json:"code" raw:"pc"`
		Quantity int    `json:"quantity" raw:"qty"`
		On       string `json:"on" raw:"on"`
		Pm       string `json:"pm" raw:"pm"`
		Pdi      string `json:"pdi" raw:"pdi"`
	}
	if err := gconv.StructTag(m, &qr, "raw"); err != nil {
		return nil
	}
	if qr.Code != "" && qr.Quantity != 0 {
		return &QRCodeData{
			Code:     qr.Code,
			Quantity: qr.Quantity,
			RawData:  c,
			Platform: "LCSC",
		}
	}
	return nil
}
