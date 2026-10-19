package mcp

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/Rehtt/hamster-bin/internal/models"
	"github.com/Rehtt/hamster-bin/internal/normalization"
	"github.com/Rehtt/hamster-bin/internal/price"
	"github.com/Rehtt/hamster-bin/internal/repository"
	"github.com/Rehtt/hamster-bin/internal/version"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"gorm.io/gorm"
)

type result struct {
	// 描述使 SDK 输出对象形式的 Schema，避免布尔 Schema 导致 Hermes 发现工具失败，同时保留任意返回值类型。
	Data       any         `json:"data" jsonschema:"工具返回的数据，结构取决于具体工具"`
	Pagination *pagination `json:"pagination,omitempty"`
}
type pagination struct {
	Page      int   `json:"page"`
	PageSize  int   `json:"page_size"`
	Total     int64 `json:"total"`
	TotalPage int64 `json:"total_page"`
}

func data(v any, err error) (result, error) { return result{Data: v}, err }
func pageResult(v any, total int64, p PageInput, err error) (result, error) {
	return result{Data: v, Pagination: &pagination{Page: p.Page, PageSize: p.PageSize, Total: total, TotalPage: (total + int64(p.PageSize) - 1) / int64(p.PageSize)}}, err
}
func (p *PageInput) normalize() error {
	if p.Page == 0 {
		p.Page = 1
	}
	if p.PageSize == 0 {
		p.PageSize = 20
	}
	if p.Page < 1 || p.PageSize < 1 {
		return fmt.Errorf("页码和每页数量必须大于零")
	}
	if p.PageSize > 100 {
		p.PageSize = 100
	}
	if p.Page-1 > math.MaxInt/p.PageSize {
		return fmt.Errorf("页码过大")
	}
	return nil
}
func register[In any](s *sdk.Server, db *gorm.DB, name, description string, readOnly, destructive bool, fn func(*gorm.DB, In) (result, error)) {
	closed := false
	sdk.AddTool(s, &sdk.Tool{Name: name, Description: description, Annotations: &sdk.ToolAnnotations{ReadOnlyHint: readOnly, DestructiveHint: &destructive, OpenWorldHint: &closed}}, func(ctx context.Context, _ *sdk.CallToolRequest, in In) (*sdk.CallToolResult, result, error) {
		out, err := fn(db.WithContext(ctx), in)
		// SDK 将业务错误转为 isError 工具结果，并自动提供结构化输出与 JSON 文本。
		return nil, out, err
	})
}
func validID(id uint) error {
	if id == 0 {
		return fmt.Errorf("ID 必须大于零")
	}
	return nil
}
func (d DetailsInput) component(name string, category uint) models.Component {
	return models.Component{ReviewConfirmations: d.ReviewConfirmations, Name: name, CategoryID: category, ComponentNumber: &d.ComponentNumber, Model: d.Model, Manufacturer: d.Manufacturer, Value: d.Value, Package: d.Package, SupplierID: d.SupplierID, SupplierPartNumber: d.SupplierPartNumber, Description: d.Description, Location: d.Location, DatasheetURL: d.DatasheetURL, ImageURL: d.ImageURL}
}

func newServer(db *gorm.DB) *sdk.Server {
	s := sdk.NewServer(&sdk.Implementation{Name: "hamster-bin", Version: version.Version}, &sdk.ServerOptions{Instructions: "管理 Hamster Bin 元件库存。总价使用整数分，单价使用微元（1 元 = 1000000 微元）。写入前请核对分类、供应商及数量。"})
	register(s, db, "list_components", "查询元件，支持分字段搜索、分类筛选、排序和分页；默认第 1 页、20 条，上限 100 条。", true, false, func(db *gorm.DB, in ListComponentsInput) (result, error) {
		if err := in.normalize(); err != nil {
			return result{}, err
		}
		if in.CategoryID != nil {
			if err := validID(*in.CategoryID); err != nil {
				return result{}, err
			}
		}
		if in.SortBy != "" && !repository.IsValidComponentSortBy(in.SortBy) {
			return result{}, fmt.Errorf("无效的排序字段")
		}
		if in.SortOrder != "" && in.SortOrder != "asc" && in.SortOrder != "desc" {
			return result{}, fmt.Errorf("排序方向必须为 asc 或 desc")
		}
		items, total, err := repository.NewComponentRepository(db).GetAll(repository.ComponentQuery{CategoryID: in.CategoryID, Keyword: in.Keyword, ComponentNumber: in.ComponentNumber, Name: in.Name, Model: in.Model, Manufacturer: in.Manufacturer, Value: in.Value, SupplierName: in.Supplier, SupplierPartNumber: in.SupplierPartNumber, Page: in.Page, PageSize: in.PageSize, SortBy: in.SortBy, SortOrder: in.SortOrder})
		return pageResult(items, total, in.PageInput, err)
	})
	register(s, db, "get_component", "按 ID 查询元件及分类、供应商资料。", true, false, func(db *gorm.DB, in IDInput) (result, error) {
		if err := validID(in.ID); err != nil {
			return result{}, err
		}
		return data(repository.NewComponentRepository(db).GetByID(in.ID))
	})
	register(s, db, "create_component", "创建元件；名称、分类 ID、非负库存必填，编号留空自动分配。有采购总价时按数量分摊单价并记录初始入库流水；总价为整数分。", false, false, func(db *gorm.DB, in CreateComponentInput) (result, error) {
		c := in.DetailsInput.component(in.Name, in.CategoryID)
		c.StockQuantity = in.StockQuantity
		return data(repository.NewComponentRepository(db).CreateWithInitialStock(&c, in.TotalPriceCents))
	})
	register(s, db, "update_component", "按 ID 局部编辑元件资料；省略字段保持原值，空字符串清除文本，supplier_id=0 解除关联。不接受库存或参考单价，库存请使用 change_stock。", false, true, func(db *gorm.DB, in UpdateComponentInput) (result, error) {
		if err := validID(in.ID); err != nil {
			return result{}, err
		}
		return data(repository.NewComponentRepository(db).PatchDetails(in.ID, in.updates(), in.ReviewConfirmations))
	})
	register(s, db, "change_stock", "变更库存：amount 正数入库、负数出库、禁止为零。入库总价使用整数分；出库成本按现有参考单价自动计算。记录流水并返回更新后的元件。", false, true, func(db *gorm.DB, in ChangeStockInput) (result, error) {
		if err := validID(in.ID); err != nil {
			return result{}, err
		}
		if in.Amount == 0 || in.Amount == math.MinInt || in.TotalPriceCents < 0 || (in.Amount < 0 && in.TotalPriceCents != 0) {
			return result{}, fmt.Errorf("数量不得为零，总价必须非负且仅用于入库")
		}
		if in.Amount > 0 && in.TotalPriceCents > (math.MaxInt64-int64(in.Amount)/2)/price.MicroPerCent {
			return result{}, fmt.Errorf("总价过大，无法安全换算为微元")
		}
		return data(repository.NewComponentRepository(db).ApplyStockChange(repository.StockChangeParams{ComponentID: in.ID, Amount: in.Amount, Reason: in.Reason, TotalPriceCents: in.TotalPriceCents, UnitPriceMicro: price.UnitPriceMicro(in.TotalPriceCents, in.Amount)}))
	})
	register(s, db, "list_pre_stocks", "分页查询预入库，状态 pending/confirmed/all，默认 pending；每页默认 20、上限 100。", true, false, func(db *gorm.DB, in ListPreStocksInput) (result, error) {
		if err := in.normalize(); err != nil {
			return result{}, err
		}
		if in.Status == "" {
			in.Status = repository.PreStockStatusPending
		}
		if in.Status != "pending" && in.Status != "confirmed" && in.Status != "all" {
			return result{}, repository.ErrInvalidPreStockStatus
		}
		items, total, err := repository.NewPreStockRepository(db).GetAll(repository.PreStockQuery{Status: in.Status, Page: in.Page, PageSize: in.PageSize})
		return pageResult(items, total, in.PageInput, err)
	})
	register(s, db, "get_pre_stock", "按 ID 查询预入库及关联资料。", true, false, func(db *gorm.DB, in IDInput) (result, error) {
		if err := validID(in.ID); err != nil {
			return result{}, err
		}
		return data(repository.NewPreStockRepository(db).GetByID(in.ID))
	})
	register(s, db, "create_pre_stock", "创建待入库记录并占用编号，不计入正式库存；名称、分类 ID、非负预计数量必填，总价为整数分。", false, false, func(db *gorm.DB, in CreatePreStockInput) (result, error) {
		c := in.DetailsInput.component(in.Name, in.CategoryID)
		p := models.PreStock{ReviewConfirmations: c.ReviewConfirmations, CategoryID: c.CategoryID, ComponentNumber: c.ComponentNumber, Name: c.Name, Model: c.Model, Manufacturer: c.Manufacturer, Value: c.Value, Package: c.Package, SupplierID: c.SupplierID, SupplierPartNumber: c.SupplierPartNumber, Description: c.Description, Location: c.Location, DatasheetURL: c.DatasheetURL, ImageURL: c.ImageURL, ExpectedQuantity: in.ExpectedQuantity, TotalPriceCents: in.TotalPriceCents}
		return data(repository.NewPreStockRepository(db).CreateValidated(&p))
	})
	register(s, db, "update_pre_stock", "局部编辑 pending 预入库；省略字段保持原值，可修改预计数量、总价（整数分）；supplier_id=0 解除关联，已确认记录不可编辑。", false, true, func(db *gorm.DB, in UpdatePreStockInput) (result, error) {
		if err := validID(in.ID); err != nil {
			return result{}, err
		}
		updates := in.updates()
		put(updates, "expected_quantity", in.ExpectedQuantity)
		put(updates, "total_price_cents", in.TotalPriceCents)
		return data(repository.NewPreStockRepository(db).PatchPending(in.ID, updates, in.ReviewConfirmations))
	})
	register(s, db, "confirm_pre_stock", "确认待入库：事务内创建正式元件、写入流水、标记已确认；同一记录只能确认一次。", false, true, func(db *gorm.DB, in ConfirmPreStockInput) (result, error) {
		if err := validID(in.ID); err != nil {
			return result{}, err
		}
		return data(repository.NewPreStockRepository(db).Confirm(in.ID, in.ReviewConfirmations))
	})
	register(s, db, "list_categories", "查询全部分类及父分类 ID。", true, false, func(db *gorm.DB, _ EmptyInput) (result, error) {
		return data(repository.NewCategoryRepository(db).GetAll())
	})
	register(s, db, "create_category", "创建分类，可关联已存在的父分类。", false, false, func(db *gorm.DB, in CreateCategoryInput) (result, error) {
		c := models.Category{Status: in.Status, Name: in.Name, ParentID: in.ParentID}
		err := repository.NewCategoryRepository(db).Create(&c)
		return data(c, err)
	})
	register(s, db, "update_category", "按 ID 局部修改分类名称或父分类；省略或 null 保留原值，parent_id=0 移为顶级分类；禁止自身、后代引用及循环。", false, true, func(db *gorm.DB, in UpdateCategoryInput) (result, error) {
		if err := validID(in.ID); err != nil {
			return result{}, err
		}
		return data(repository.NewCategoryRepository(db).Update(in.ID, repository.CategoryPatch{Status: in.Status, Name: in.Name, ParentID: in.ParentID}))
	})
	register(s, db, "delete_category", "按 ID 删除空分类；关联元件（包括零库存）、任意状态的预入库记录或子分类时拒绝删除，不迁移或级联删除关联数据。", false, true, func(db *gorm.DB, in IDInput) (result, error) {
		if err := validID(in.ID); err != nil {
			return result{}, err
		}
		err := repository.NewCategoryRepository(db).Delete(in.ID)
		return data(map[string]any{"id": in.ID, "message": "删除成功"}, err)
	})
	register(s, db, "list_suppliers", "按名称查询全部供应商。", true, false, func(db *gorm.DB, _ EmptyInput) (result, error) {
		return data(repository.NewSupplierRepository(db).GetAll())
	})
	register(s, db, "create_supplier", "按名称创建供应商，同名时返回已有记录；使用返回的 ID 关联元件或预入库。", false, false, func(db *gorm.DB, in CreateSupplierInput) (result, error) {
		if strings.TrimSpace(in.Name) == "" {
			return result{}, fmt.Errorf("供应商名称不能为空")
		}
		return data(repository.NewSupplierRepository(db).FirstOrCreateByName(in.Name))
	})
	register(s, db, "list_stock_logs", "分页查询库存流水，默认第 1 页、20 条，上限 100 条。", true, false, func(db *gorm.DB, in PageInput) (result, error) {
		if err := in.normalize(); err != nil {
			return result{}, err
		}
		items, total, err := repository.NewStockLogRepository(db).GetAll(in.Page, in.PageSize)
		return pageResult(items, total, in, err)
	})
	register(s, db, "get_component_stock_logs", "查询指定元件最近流水，默认 50 条，上限 100 条。", true, false, func(db *gorm.DB, in ComponentLogsInput) (result, error) {
		if err := validID(in.ID); err != nil {
			return result{}, err
		}
		if in.Limit == 0 {
			in.Limit = 50
		}
		if in.Limit < 0 {
			return result{}, fmt.Errorf("limit 必须大于零")
		}
		if in.Limit > 100 {
			in.Limit = 100
		}
		if _, err := repository.NewComponentRepository(db).GetByID(in.ID); err != nil {
			return result{}, err
		}
		return data(repository.NewStockLogRepository(db).GetByComponentID(in.ID, in.Limit))
	})
	register(s, db, "get_stats", "查询库存及入出库统计；range 可选 month/quarter/all，默认 month。金额为整数分。", true, false, func(db *gorm.DB, in StatsInput) (result, error) {
		if in.Range == "" {
			in.Range = repository.StatsRangeMonth
		}
		return data(repository.NewStatsRepository(db).GetDashboardStats(in.Range))
	})
	register(s, db, "normalization_standards", "读取版本化正式规范及独立历史候选；历史值不用于自动规范化。", true, false, func(db *gorm.DB, _ EmptyInput) (result, error) {
		return data(repository.NewNormalizationRepository(db).Standards(db.Statement.Context))
	})
	register(s, db, "normalization_preview", "本地确定性预览，不保存、不联网；返回候选、变化及问题。", true, false, func(db *gorm.DB, in PreviewNormalizationInput) (result, error) {
		s, err := repository.NewNormalizationRepository(db).Standards(db.Statement.Context)
		if err != nil {
			return result{}, err
		}
		return data(normalization.Preview(normalization.Fields{Name: in.Name, Description: in.Description, Model: in.Model, Manufacturer: in.Manufacturer, Package: in.Package, CategoryName: in.CategoryName, CategoryID: in.CategoryID}, s), nil)
	})
	return s
}
