package parser

import (
	"context"
	"encoding/json"
	"github.com/Rehtt/hamster-bin/internal/normalization"
	"sort"
	"time"
	"unicode/utf8"
)

type standardsKey struct{}

func standardsFrom(ctx context.Context) normalization.Standards {
	if s, ok := ctx.Value(standardsKey{}).(normalization.Standards); ok {
		return s
	}
	return normalization.Rules()
}
func (m *ParserManager) SetStandardsReader(reader normalization.Reader) {
	m.standardsMu.Lock()
	defer m.standardsMu.Unlock()
	m.standards = reader
}
func (m *ParserManager) readStandards(ctx context.Context) (normalization.Standards, error) {
	m.standardsMu.RLock()
	reader := m.standards
	m.standardsMu.RUnlock()
	if reader != nil {
		return reader.Standards(ctx)
	}
	return normalization.Rules(), nil
}
func infoFields(i *ComponentInfo) normalization.Fields {
	return normalization.Fields{Name: i.Name, Description: i.Description, Model: i.Model, Manufacturer: i.Manufacturer, Package: i.Package, CategoryName: i.CategoryName, CategoryID: i.CategoryID, Value: i.Value, DatasheetURL: i.DatasheetURL, ImageURL: i.ImageURL, PlatformCode: i.PlatformCode}
}
func snapshot(i *ComponentInfo, name, url, params string) normalization.Source {
	params, truncated := limitParameters(params)
	return normalization.Source{Name: name, URL: url, QueriedAt: time.Now().UTC().Format(time.RFC3339Nano), Fields: infoFields(i), Parameters: params, Truncated: truncated}
}
func normalizeInfo(i *ComponentInfo, s normalization.Standards) {
	if i == nil || i.Normalization != nil {
		return
	}
	if len(i.sources) == 0 {
		i.sources = []normalization.Source{snapshot(i, i.PlatformName, i.PlatformURL, "")}
	}
	sort.Slice(i.sources, func(a, b int) bool { return i.sources[a].Name < i.sources[b].Name })
	candidate := infoFields(i)
	result := normalization.Preview(candidate, s)
	result.Normalization.Sources = i.sources
	result.Normalization.PrimarySource = i.primarySource
	conflicts := normalization.Conflicts(i.sources, s)
	result.Normalization.Issues = append(result.Normalization.Issues, conflicts...)
	// The chosen source owns conflicting facts; never silently combine other queried sources.
	primary := i.sources[0].Fields
	for _, src := range i.sources {
		if src.Name == i.primarySource {
			primary = src.Fields
		}
	}
	for _, conflict := range conflicts {
		switch conflict.Field {
		case "name":
			i.Name = primary.Name
		case "description":
			i.Description = primary.Description
		case "model":
			i.Model = primary.Model
		case "value":
			i.Value = primary.Value
		case "package":
			i.Package = primary.Package
		case "manufacturer":
			i.Manufacturer = primary.Manufacturer
		case "category_name":
			i.CategoryName = primary.CategoryName
		}
	}
	if len(conflicts) > 0 {
		preview := normalization.Preview(infoFields(i), s)
		result.Candidate = preview.Candidate
		result.Normalization.Changes = preview.Normalization.Changes
		result.Normalization.Issues = append(preview.Normalization.Issues, conflicts...)
	}
	i.Name = result.Candidate.Name
	i.Description = result.Candidate.Description
	i.Package = result.Candidate.Package
	i.Manufacturer = result.Candidate.Manufacturer
	i.CategoryName = result.Candidate.CategoryName
	i.CategoryID = result.Candidate.CategoryID
	if i.semantic {
		baseline := normalization.Preview(primary, s).Candidate
		result.Normalization.Changes = []normalization.Change{}
		for _, pair := range []struct{ k, original, candidate, deterministic string }{
			{"name", primary.Name, i.Name, baseline.Name}, {"description", primary.Description, i.Description, baseline.Description}, {"model", primary.Model, i.Model, baseline.Model}, {"value", primary.Value, i.Value, baseline.Value}, {"package", primary.Package, i.Package, baseline.Package}, {"manufacturer", primary.Manufacturer, i.Manufacturer, baseline.Manufacturer}, {"category_name", primary.CategoryName, i.CategoryName, baseline.CategoryName}, {"datasheet_url", primary.DatasheetURL, i.DatasheetURL, baseline.DatasheetURL}, {"image_url", primary.ImageURL, i.ImageURL, baseline.ImageURL},
		} {
			if pair.original != pair.candidate {
				semantic := pair.candidate != pair.deterministic
				result.Normalization.Changes = append(result.Normalization.Changes, normalization.Change{Field: pair.k, Original: pair.original, Candidate: pair.candidate, Semantic: semantic})
				if semantic {
					result.Normalization.Issues = append(result.Normalization.Issues, normalization.Issue{Field: pair.k, Code: "semantic_candidate", Message: "AI 整理候选，需人工确认，未经确定性事实验证"})
				}
			}
		}
	}

	i.Normalization = &result.Normalization
}
func normalizationPrompt(ctx context.Context) string {
	s := standardsFrom(ctx)
	categories, _ := json.Marshal(s.Categories)
	rules, _ := json.Marshal(normalization.Rules())
	return "\n仅在已有来源内整理名称和描述为中文，不补充额定值、功能或用途；未知保留原文。型号、品牌、单位及 MOSFET、USB Type-C、EEPROM 保持原样。厂商别名为空，禁止按名称相似合并公司。封装只允许四条精确映射，不推断其他后缀。分类仅可精确选择下列启用分类，无法匹配留空。版本化规则：" + string(rules) + "\n启用分类：" + string(categories)
}

func limitParameters(value string) (string, bool) {
	if len(value) <= 12000 {
		return value, false
	}
	value = value[:12000]
	for !utf8.ValidString(value) {
		value = value[:len(value)-1]
	}
	return value, true
}
