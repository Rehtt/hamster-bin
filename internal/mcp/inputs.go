package mcp

// 参数类型用于 SDK 自动生成 JSON Schema；omitempty 表示调用方可省略。
type PageInput struct {
	Page     int `json:"page,omitempty" jsonschema:"页码，默认 1，必须为正整数"`
	PageSize int `json:"page_size,omitempty" jsonschema:"每页数量，默认 20，上限 100"`
}
type IDInput struct {
	ID uint `json:"id" jsonschema:"记录 ID，必须大于零"`
}
type EmptyInput struct{}
type ListComponentsInput struct {
	PageInput
	CategoryID         *uint  `json:"category_id,omitempty" jsonschema:"按分类 ID 筛选"`
	Keyword            string `json:"keyword,omitempty" jsonschema:"兼容综合搜索，空格分词且各词均须匹配"`
	ComponentNumber    string `json:"component_number,omitempty" jsonschema:"元件编号，空格分词 AND 匹配"`
	Name               string `json:"name,omitempty" jsonschema:"名称，空格分词 AND 匹配"`
	Model              string `json:"model,omitempty" jsonschema:"厂家型号"`
	Manufacturer       string `json:"manufacturer,omitempty" jsonschema:"制造商"`
	Value              string `json:"value,omitempty" jsonschema:"参数值"`
	Supplier           string `json:"supplier,omitempty" jsonschema:"供应商名称"`
	SupplierPartNumber string `json:"supplier_part_number,omitempty" jsonschema:"供应商料号"`
	SortBy             string `json:"sort_by,omitempty" jsonschema:"排序字段，默认 updated_at；支持 component_number/name/model/manufacturer/value/package/description/category/stock_quantity/unit_price/location/supplier/supplier_part_number/datasheet_url/created_at/updated_at"`
	SortOrder          string `json:"sort_order,omitempty" jsonschema:"排序方向 asc 或 desc，默认 desc"`
}
type DetailsInput struct {
	ReviewConfirmations map[string]string `json:"review_confirmations,omitempty" jsonschema:"明确确认保留原文，键为 name/description，值必须等于本次提交字段值"`
	ComponentNumber     string            `json:"component_number,omitempty" jsonschema:"唯一元件编号，留空自动分配 HB-xxxxxx"`
	Model               string            `json:"model,omitempty" jsonschema:"厂家型号"`
	Manufacturer        string            `json:"manufacturer,omitempty" jsonschema:"制造商"`
	Value               string            `json:"value,omitempty" jsonschema:"参数值"`
	Package             string            `json:"package,omitempty" jsonschema:"封装"`
	SupplierID          *uint             `json:"supplier_id,omitempty" jsonschema:"已存在的供应商 ID，可先创建供应商"`
	SupplierPartNumber  string            `json:"supplier_part_number,omitempty" jsonschema:"供应商料号"`
	Description         string            `json:"description,omitempty" jsonschema:"描述"`
	Location            string            `json:"location,omitempty" jsonschema:"存放位置"`
	DatasheetURL        string            `json:"datasheet_url,omitempty" jsonschema:"资料链接"`
	ImageURL            string            `json:"image_url,omitempty" jsonschema:"图片链接"`
}
type CreateComponentInput struct {
	DetailsInput
	Name            string `json:"name" jsonschema:"元件名称，不能为空"`
	CategoryID      uint   `json:"category_id" jsonschema:"已存在的分类 ID"`
	StockQuantity   int    `json:"stock_quantity" jsonschema:"初始库存数量，必须非负"`
	TotalPriceCents int64  `json:"total_price_cents,omitempty" jsonschema:"采购总价，单位整数分，必须非负；正数总价要求数量大于零"`
}
type CreatePreStockInput struct {
	DetailsInput
	Name             string `json:"name" jsonschema:"元件名称，不能为空"`
	CategoryID       uint   `json:"category_id" jsonschema:"已存在的分类 ID"`
	ExpectedQuantity int    `json:"expected_quantity" jsonschema:"预计数量，必须非负"`
	TotalPriceCents  int64  `json:"total_price_cents,omitempty" jsonschema:"采购总价，单位整数分，必须非负；正数总价要求数量大于零"`
}

// 指针区分省略与零值；供应商 ID 为 0 表示解除关联。
type PatchDetailsInput struct {
	ReviewConfirmations map[string]string `json:"review_confirmations,omitempty" jsonschema:"保留原文的逐字段确认，值须等于本次提交值"`
	Name                *string           `json:"name,omitempty" jsonschema:"元件名称，提供时不能为空"`
	CategoryID          *uint             `json:"category_id,omitempty" jsonschema:"已存在的分类 ID"`
	ComponentNumber     *string           `json:"component_number,omitempty" jsonschema:"唯一元件编号"`
	Model               *string           `json:"model,omitempty" jsonschema:"厂家型号，可用空字符串清除"`
	Manufacturer        *string           `json:"manufacturer,omitempty" jsonschema:"制造商"`
	Value               *string           `json:"value,omitempty" jsonschema:"参数值"`
	Package             *string           `json:"package,omitempty" jsonschema:"封装"`
	SupplierID          *uint             `json:"supplier_id,omitempty" jsonschema:"供应商 ID，0 表示解除关联"`
	SupplierPartNumber  *string           `json:"supplier_part_number,omitempty" jsonschema:"供应商料号"`
	Description         *string           `json:"description,omitempty" jsonschema:"描述"`
	Location            *string           `json:"location,omitempty" jsonschema:"存放位置"`
	DatasheetURL        *string           `json:"datasheet_url,omitempty" jsonschema:"资料链接"`
	ImageURL            *string           `json:"image_url,omitempty" jsonschema:"图片链接"`
}
type UpdateComponentInput struct {
	IDInput
	PatchDetailsInput
}
type UpdatePreStockInput struct {
	IDInput
	PatchDetailsInput
	ExpectedQuantity *int   `json:"expected_quantity,omitempty" jsonschema:"预计数量，必须非负"`
	TotalPriceCents  *int64 `json:"total_price_cents,omitempty" jsonschema:"采购总价，整数分，必须非负"`
}
type ChangeStockInput struct {
	IDInput
	Amount          int    `json:"amount" jsonschema:"库存变化数量，正数入库、负数出库，禁止为零"`
	TotalPriceCents int64  `json:"total_price_cents,omitempty" jsonschema:"入库总价，整数分且非负；出库时不得提供正数总价，成本自动计算"`
	Reason          string `json:"reason,omitempty" jsonschema:"库存变更原因"`
}
type ListPreStocksInput struct {
	PageInput
	Status string `json:"status,omitempty" jsonschema:"pending/confirmed/all，默认 pending"`
}
type ComponentLogsInput struct {
	IDInput
	Limit int `json:"limit,omitempty" jsonschema:"最近流水条数，默认 50，上限 100"`
}
type StatsInput struct {
	Range string `json:"range,omitempty" jsonschema:"统计时间范围 month/quarter/all，默认 month"`
}
type CreateCategoryInput struct {
	Status   string `json:"status,omitempty" jsonschema:"pending/active/inactive，默认 pending"`
	Name     string `json:"name" jsonschema:"分类名称，不能为空"`
	ParentID *uint  `json:"parent_id,omitempty" jsonschema:"已存在的父分类 ID，不填或 0 表示顶级分类"`
}
type UpdateCategoryInput struct {
	Status *string `json:"status,omitempty" jsonschema:"pending/active/inactive，改名后强制 pending"`
	IDInput
	Name     *string `json:"name,omitempty" jsonschema:"分类名称，去除首尾空白后不能为空；省略或 null 保持原值"`
	ParentID *uint   `json:"parent_id,omitempty" jsonschema:"已存在的父分类 ID；0 表示移为顶级分类，省略或 null 保持原值；不能是自身或后代"`
}
type CreateSupplierInput struct {
	Name string `json:"name" jsonschema:"供应商名称，不能为空；同名时返回已有供应商"`
}

func put[T any](m map[string]any, key string, value *T) {
	if value != nil {
		m[key] = *value
	}
}
func (p PatchDetailsInput) updates() map[string]any {
	m := map[string]any{}
	put(m, "name", p.Name)
	put(m, "category_id", p.CategoryID)
	put(m, "component_number", p.ComponentNumber)
	put(m, "model", p.Model)
	put(m, "manufacturer", p.Manufacturer)
	put(m, "value", p.Value)
	put(m, "package", p.Package)
	put(m, "supplier_part_number", p.SupplierPartNumber)
	put(m, "description", p.Description)
	put(m, "location", p.Location)
	put(m, "datasheet_url", p.DatasheetURL)
	put(m, "image_url", p.ImageURL)
	if p.SupplierID != nil {
		if *p.SupplierID == 0 {
			m["supplier_id"] = nil
		} else {
			m["supplier_id"] = *p.SupplierID
		}
	}
	return m
}

type PreviewNormalizationInput struct {
	Name         string `json:"name,omitempty"`
	Description  string `json:"description,omitempty"`
	Model        string `json:"model,omitempty"`
	Manufacturer string `json:"manufacturer,omitempty"`
	Package      string `json:"package,omitempty"`
	CategoryName string `json:"category_name,omitempty"`
	CategoryID   uint   `json:"category_id,omitempty"`
}
type ConfirmPreStockInput struct {
	IDInput
	ReviewConfirmations map[string]string `json:"review_confirmations,omitempty"`
}
