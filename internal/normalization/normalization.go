// Package normalization owns versioned, deterministic inventory rules. It performs no I/O.
package normalization

import (
	"context"
	"crypto/sha256"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

const Version = "2026-10-19.1"

type Fields struct {
	SupplierName       string `json:"supplier_name,omitempty"`
	SupplierPartNumber string `json:"supplier_part_number,omitempty"`
	Quantity           *int   `json:"quantity,omitempty"`
	Value              string `json:"value,omitempty"`
	DatasheetURL       string `json:"datasheet_url,omitempty"`
	ImageURL           string `json:"image_url,omitempty"`
	PlatformCode       string `json:"platform_code,omitempty"`
	Name               string `json:"name"`
	Description        string `json:"description"`
	Model              string `json:"model"`
	Manufacturer       string `json:"manufacturer"`
	Package            string `json:"package"`
	CategoryName       string `json:"category_name"`
	CategoryID         uint   `json:"category_id,omitempty"`
}
type Category struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}
type Standards struct {
	Version             string              `json:"version"`
	Categories          []Category          `json:"categories"`
	CategoryAliases     map[string]string   `json:"category_aliases"`
	ManufacturerAliases map[string]string   `json:"manufacturer_aliases"`
	Packages            map[string]string   `json:"packages"`
	Dictionary          map[string]string   `json:"dictionary"`
	Historical          map[string][]string `json:"historical_candidates"`
}
type Reader interface {
	Standards(context.Context) (Standards, error)
}
type Source struct {
	Name       string `json:"name"`
	URL        string `json:"url,omitempty"`
	QueriedAt  string `json:"queried_at"`
	Fields     Fields `json:"fields"`
	Parameters string `json:"parameters,omitempty"`
	Truncated  bool   `json:"truncated"`
}
type Change struct {
	Field     string `json:"field"`
	Original  string `json:"original"`
	Candidate string `json:"candidate"`
	Semantic  bool   `json:"semantic,omitempty"`
}
type Issue struct {
	Field   string   `json:"field"`
	Code    string   `json:"code"`
	Message string   `json:"message"`
	Sources []string `json:"sources,omitempty"`
}
type Metadata struct {
	PrimarySource string   `json:"primary_source,omitempty"`
	Version       string   `json:"version"`
	Sources       []Source `json:"sources"`
	Changes       []Change `json:"changes"`
	Issues        []Issue  `json:"issues"`
}
type Result struct {
	Candidate     Fields   `json:"candidate"`
	Normalization Metadata `json:"normalization"`
}
type Review struct {
	Field     string `json:"field"`
	Reason    string `json:"reason"`
	Confirmed bool   `json:"confirmed"`
	ValueHash string `json:"value_hash"`
}
type ValidationError struct {
	Issues []Issue `json:"issues"`
	Item   int     `json:"item,omitempty"`
}

func (e *ValidationError) Error() string {
	prefix := ""
	if e.Item > 0 {
		prefix = fmt.Sprintf("第 %d 条：", e.Item)
	}
	messages := []string{}
	for _, i := range e.Issues {
		messages = append(messages, i.Field+": "+i.Message)
	}
	return prefix + strings.Join(messages, "；")
}
func Invalid(field, code, message string) error {
	return &ValidationError{Issues: []Issue{{Field: field, Code: code, Message: message}}}
}
func Rules() Standards {
	return Standards{Version: Version, Categories: []Category{}, CategoryAliases: map[string]string{}, ManufacturerAliases: map[string]string{}, Packages: map[string]string{"SOT23": "SOT-23", "SOT-23": "SOT-23", "SOT23-5": "SOT-23-5", "SOT-23-5": "SOT-23-5"}, Dictionary: map[string]string{"Resistor": "电阻", "Resistors": "电阻", "Capacitor": "电容", "Capacitors": "电容", "Inductor": "电感", "Inductors": "电感", "Diode": "二极管", "Diodes": "二极管", "Transistor": "晶体管", "Transistors": "晶体管", "Connector": "连接器", "Connectors": "连接器"}, Historical: map[string][]string{}}
}
func Package(value string) string {
	v := strings.TrimSpace(value)
	if mapped, ok := Rules().Packages[v]; ok {
		return mapped
	}
	return v
}
func translate(v string, s Standards) string {
	if mapped, ok := s.Dictionary[v]; ok {
		return mapped
	}
	return v
}

var words = regexp.MustCompile(`[A-Za-z][A-Za-z0-9_-]*`)
var modelToken = regexp.MustCompile(`^[A-Z0-9_-]*[0-9][A-Z0-9_-]*$`)
var numericUnits = regexp.MustCompile(`[0-9]+(?:\.[0-9]+)?\s*[pnumkMGT]?(?:Ω|Ohm|ohm|V|A|W|Hz|F|H|s|m)`)
var units = regexp.MustCompile(`^[pnumkMGT]?(V|A|W|Hz|F|H|Ohm|ohm|Ω|s|m)$`)

// NeedsReview recognizes words, excluding explicit technical identifiers, brand/model and units.
func NeedsReview(value, model, manufacturer string) bool {
	value = strings.ReplaceAll(value, "USB Type-C", "")
	value = numericUnits.ReplaceAllString(value, "")
	for _, literal := range []string{model, manufacturer} {
		if literal != "" {
			value = strings.ReplaceAll(value, literal, "")
		}
	}
	allowed := map[string]bool{"MOSFET": true, "USB": true, "EEPROM": true, "MCU": true, "LED": true, "IC": true, "PCB": true, "CPU": true, "RAM": true, "ROM": true, "SMD": true, "DC": true, "AC": true, "I2C": true, "SPI": true, "UART": true, "NPN": true, "PNP": true, "CMOS": true, "ESD": true, "LDO": true, "RF": true}
	for _, w := range words.FindAllString(value, -1) {
		if allowed[w] || units.MatchString(w) {
			continue
		}
		if modelToken.MatchString(w) {
			continue
		}
		return true
	}
	return false
}
func Preview(f Fields, s Standards) Result {
	original := f
	f.Name = translate(f.Name, s)
	f.Description = translate(f.Description, s)
	f.Package = Package(f.Package)
	if v, ok := s.ManufacturerAliases[f.Manufacturer]; ok {
		f.Manufacturer = v
	}
	category := translate(f.CategoryName, s)
	if v, ok := s.CategoryAliases[category]; ok {
		category = v
	}
	f.CategoryID = 0
	matches := []Category{}
	for _, c := range s.Categories {
		if (original.CategoryID > 0 && c.ID == original.CategoryID) || (original.CategoryID == 0 && category != "" && c.Name == category) {
			matches = append(matches, c)
		}
	}
	if len(matches) == 1 {
		f.CategoryID = matches[0].ID
		f.CategoryName = matches[0].Name
	} else {
		f.CategoryName = ""
	}
	meta := Metadata{Version: Version, Sources: []Source{}, Changes: []Change{}, Issues: []Issue{}}
	for _, pair := range []struct{ k, a, b string }{{"name", original.Name, f.Name}, {"description", original.Description, f.Description}, {"package", original.Package, f.Package}, {"manufacturer", original.Manufacturer, f.Manufacturer}, {"category_name", original.CategoryName, f.CategoryName}} {
		if pair.a != pair.b {
			meta.Changes = append(meta.Changes, Change{Field: pair.k, Original: pair.a, Candidate: pair.b})
		}
	}
	for _, pair := range []struct{ k, v string }{{"name", f.Name}, {"description", f.Description}} {
		if NeedsReview(pair.v, f.Model, f.Manufacturer) {
			meta.Issues = append(meta.Issues, Issue{Field: pair.k, Code: "needs_review", Message: "无法可靠中文化，请整理或明确确认保留原文"})
		}
	}
	if f.CategoryID == 0 {
		meta.Issues = append(meta.Issues, Issue{Field: "category_id", Code: "category_unmatched", Message: "请选择经审核启用的分类"})
	}
	if f.Package != "" {
		if _, ok := s.Packages[f.Package]; !ok {
			meta.Issues = append(meta.Issues, Issue{Field: "package", Code: "unmapped_package", Message: "未应用封装映射，允许保留"})
		}
	}
	return Result{Candidate: f, Normalization: meta}
}
func Hash(v string) string { return fmt.Sprintf("%x", sha256.Sum256([]byte(v))) }

// ReviewFields never translates. Unchanged historical fields retain their existing review state.
func ReviewFields(f, old Fields, previous []Review, confirmations map[string]string, creating bool) ([]Review, error) {
	reviews := []Review{}
	issues := []Issue{}
	for _, p := range []struct{ k, v, old string }{{"name", f.Name, old.Name}, {"description", f.Description, old.Description}} {
		if !creating && p.v == p.old {
			if value, ok := confirmations[p.k]; ok && value == p.v && NeedsReview(p.v, f.Model, f.Manufacturer) {
				reviews = append(reviews, Review{Field: p.k, Reason: "needs_review", Confirmed: true, ValueHash: Hash(p.v)})
				continue
			}
			for _, r := range previous {
				if r.Field == p.k && r.ValueHash == Hash(p.v) {
					reviews = append(reviews, r)
				}
			}
			continue
		}
		if NeedsReview(p.v, f.Model, f.Manufacturer) {
			confirmed, valuePresent := confirmations[p.k]
			if !valuePresent || confirmed != p.v {
				issues = append(issues, Issue{Field: p.k, Code: "confirmation_required", Message: "请明确确认保留本次提交的原文"})
			}
			reviews = append(reviews, Review{Field: p.k, Reason: "needs_review", Confirmed: valuePresent && confirmed == p.v, ValueHash: Hash(p.v)})
		}
	}
	if len(issues) > 0 {
		return nil, &ValidationError{Issues: issues}
	}
	return reviews, nil
}

// Conflicts sorts sources, so query order cannot influence conflict reporting.
func Conflicts(sources []Source, s Standards) []Issue {
	sources = append([]Source(nil), sources...)
	sort.Slice(sources, func(i, j int) bool { return sources[i].Name < sources[j].Name })
	issues := []Issue{}
	for _, field := range []string{"name", "description", "model", "manufacturer", "package", "category_name", "value"} {
		values := map[string][]string{}
		for _, src := range sources {
			f := Preview(src.Fields, s).Candidate
			v := map[string]string{"name": f.Name, "description": f.Description, "model": f.Model, "manufacturer": f.Manufacturer, "package": f.Package, "category_name": f.CategoryName, "value": f.Value}[field]
			if v != "" {
				values[v] = append(values[v], src.Name)
			}
		}
		if len(values) > 1 {
			names := []string{}
			for _, src := range sources {
				names = append(names, src.Name)
			}
			issues = append(issues, Issue{Field: field, Code: "source_conflict", Message: "已查询来源存在冲突，请对照主来源确认", Sources: names})
		}
	}
	return issues
}
