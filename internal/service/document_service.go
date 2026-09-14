package service

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/extrame/xls"
	"github.com/fumiama/go-docx"
	"github.com/ledongthuc/pdf"
	"github.com/xuri/excelize/v2"
)

// ParsedDocument is what the LLM / tools receive from the document parser.
// Tables are rendered as `[]string` per row (CSV-style); full text is a single
// concatenated block; preview is the first ~2k chars for a quick glance.
type ParsedDocument struct {
	FileName    string       `json:"file_name"`
	Size        int64        `json:"size"`
	Kind        string       `json:"kind"` // text / csv / table / docx / pdf / json
	Pages       int          `json:"pages,omitempty"`
	Sheets      []string     `json:"sheets,omitempty"`
	Paragraphs  []string     `json:"paragraphs,omitempty"`
	Tables      [][][]string `json:"tables,omitempty"` // [tableIdx][rowIdx][colIdx]
	FullText    string       `json:"full_text"`
	Preview     string       `json:"preview"`
	ContentType string       `json:"content_type"`
	ParsedAt    time.Time    `json:"parsed_at"`
}

// DocumentService exposes parsers as a domain service so the assistant tools
// can call them mid-loop (not only at upload time) and the same logic is
// reusable for the REST / MCP layers later.
type DocumentService struct {
	mu sync.RWMutex
}

func NewDocumentService() *DocumentService { return &DocumentService{} }

// maxParseBytes prevents an OOM on a malicious multi-MB file being inlined
// whole into the LLM context. Files larger than this still parse, but the
// returned FullText is truncated and a notice is appended.
const maxParseBytes = 2 << 20 // 2 MB

func (d *DocumentService) ParseFile(ctx context.Context, storedPath, originalName string) (*ParsedDocument, error) {
	info, err := os.Stat(storedPath)
	if err != nil {
		return nil, fmt.Errorf("stat file: %w", err)
	}
	ext := strings.ToLower(filepath.Ext(originalName))
	if originalName == "" {
		ext = strings.ToLower(filepath.Ext(storedPath))
	}
	var parsed *ParsedDocument
	switch ext {
	case ".pdf":
		parsed, err = d.parsePDF(storedPath)
	case ".docx":
		parsed, err = d.parseDOCX(storedPath)
	case ".xlsx":
		parsed, err = d.parseXLSX(storedPath)
	case ".xls":
		parsed, err = d.parseXLS(storedPath)
	case ".csv", ".tsv":
		parsed, err = d.parseCSV(storedPath, ext == ".tsv")
	case ".json":
		parsed, err = d.parseJSON(storedPath)
	case ".txt", ".md", ".log", ".markdown":
		parsed, err = d.parseText(storedPath, strings.TrimPrefix(ext, "."))
	default:
		// Fallback: try to read as plain text.
		parsed, err = d.parseText(storedPath, "plain")
	}
	if err != nil {
		return nil, err
	}
	parsed.FileName = originalName
	if parsed.FileName == "" {
		parsed.FileName = filepath.Base(storedPath)
	}
	parsed.Size = info.Size()
	parsed.ParsedAt = time.Now()
	if len(parsed.FullText) > maxParseBytes {
		parsed.FullText = parsed.FullText[:maxParseBytes] +
			fmt.Sprintf("\n\n[...文本已截断，原文件 %d 字节超出 %d 字节上限...]", info.Size(), maxParseBytes)
	}
	previewLen := 2000
	if len(parsed.FullText) < previewLen {
		previewLen = len(parsed.FullText)
	}
	parsed.Preview = parsed.FullText[:previewLen]
	if parsed.ContentType == "" {
		parsed.ContentType = "text/plain"
	}
	return parsed, nil
}

// Summary turns a ParsedDocument into a compact (~1500 tokens) summary string
// that is cheap to drop into a system prompt. Tables over 10 rows are shrunk
// to header + 3 samples.
func (d *DocumentService) Summary(p *ParsedDocument, maxChars int) string {
	if maxChars <= 0 {
		maxChars = 6000
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# 文档摘要：%s\n- 类型: %s  大小: %d 字节", p.FileName, p.Kind, p.Size)
	if p.Pages > 0 {
		fmt.Fprintf(&b, "  页数: %d", p.Pages)
	}
	if len(p.Sheets) > 0 {
		fmt.Fprintf(&b, "  工作表: %v", p.Sheets)
	}
	b.WriteString("\n\n")
	if len(p.Paragraphs) > 0 {
		b.WriteString("## 关键段落（前 10 条）\n")
		n := 10
		if len(p.Paragraphs) < n {
			n = len(p.Paragraphs)
		}
		for i := 0; i < n; i++ {
			line := strings.TrimSpace(p.Paragraphs[i])
			if line == "" {
				continue
			}
			if len(line) > 200 {
				line = line[:200] + "…"
			}
			fmt.Fprintf(&b, "- %s\n", line)
		}
		b.WriteString("\n")
	}
	if len(p.Tables) > 0 {
		b.WriteString("## 表格\n")
		for ti, t := range p.Tables {
			fmt.Fprintf(&b, "### 表 %d（共 %d 行）\n", ti+1, len(t))
			if len(t) == 0 {
				continue
			}
			head := t[0]
			sampleRows := t[1:]
			if len(sampleRows) > 3 {
				sampleRows = sampleRows[:3]
			}
			fmt.Fprintf(&b, "列名: %s\n", strings.Join(head, " | "))
			for _, r := range sampleRows {
				for i := range r {
					if len(r[i]) > 40 {
						r[i] = r[i][:40] + "…"
					}
				}
				fmt.Fprintf(&b, " - %s\n", strings.Join(r, " | "))
			}
			if len(t)-1 > 3 {
				fmt.Fprintf(&b, " （还有 %d 行省略…）\n", len(t)-1-3)
			}
			b.WriteString("\n")
		}
	}
	if b.Len() < maxChars && len(p.FullText) > 0 && len(p.Paragraphs) == 0 && len(p.Tables) == 0 {
		rest := maxChars - b.Len()
		if rest > 0 {
			ft := p.FullText
			if len(ft) > rest {
				ft = ft[:rest]
			}
			fmt.Fprintf(&b, "## 全文片段\n%s", ft)
		}
	}
	out := b.String()
	if len(out) > maxChars {
		out = out[:maxChars] + "…"
	}
	return out
}

// ListSessionAttachments returns recent attachments in the upload directory
// so the LLM can "see" which files are already available in this session.
// It's a lightweight scan; detailed metadata is provided by parse per-file.
type AttachmentStub struct {
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	Size      int64     `json:"size"`
	ModTime   time.Time `json:"mtime"`
	Extension string    `json:"ext"`
}

func (d *DocumentService) ListSessionAttachments(uploadDir string, max int) ([]AttachmentStub, error) {
	if uploadDir == "" {
		return nil, errors.New("upload dir not configured")
	}
	entries, err := os.ReadDir(uploadDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []AttachmentStub{}, nil
		}
		return nil, err
	}
	stubs := make([]AttachmentStub, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		stubs = append(stubs, AttachmentStub{
			Name:      e.Name(),
			Path:      filepath.Join(uploadDir, e.Name()),
			Size:      info.Size(),
			ModTime:   info.ModTime(),
			Extension: strings.ToLower(filepath.Ext(e.Name())),
		})
	}
	sort.Slice(stubs, func(i, j int) bool { return stubs[i].ModTime.After(stubs[j].ModTime) })
	if max > 0 && len(stubs) > max {
		stubs = stubs[:max]
	}
	return stubs, nil
}

// ----------------- per-format parsers -----------------

func (d *DocumentService) parseText(path, kind string) (*ParsedDocument, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	text := string(raw)
	paragraphs := splitParagraphs(text)
	p := &ParsedDocument{
		Kind:        kind,
		FullText:    text,
		Paragraphs:  paragraphs,
		ContentType: "text/plain",
	}
	return p, nil
}

func (d *DocumentService) parseJSON(path string) (*ParsedDocument, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, raw, "", "  "); err == nil {
		raw = pretty.Bytes()
	}
	var anyVal any
	_ = json.Unmarshal(raw, &anyVal)
	paragraphs := []string{string(raw)}
	var tables [][][]string
	if arr, ok := anyVal.([]any); ok && len(arr) > 0 {
		if m, ok := arr[0].(map[string]any); ok {
			keys := sortedKeys(m)
			tbl := [][]string{keys}
			for _, row := range arr {
				if rm, ok := row.(map[string]any); ok {
					out := make([]string, 0, len(keys))
					for _, k := range keys {
						out = append(out, stringifyJSON(rm[k]))
					}
					tbl = append(tbl, out)
				}
			}
			tables = append(tables, tbl)
		}
	}
	return &ParsedDocument{
		Kind:        "json",
		FullText:    string(raw),
		Paragraphs:  paragraphs,
		Tables:      tables,
		ContentType: "application/json",
	}, nil
}

func (d *DocumentService) parseCSV(path string, tsv bool) (*ParsedDocument, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	if tsv {
		r.Comma = '\t'
	}
	r.LazyQuotes = true
	r.FieldsPerRecord = -1
	rows, err := r.ReadAll()
	if err != nil && len(rows) == 0 {
		return nil, err
	}
	full := renderTableAsText(rows)
	return &ParsedDocument{
		Kind:        "csv",
		Tables:      [][][]string{rows},
		Paragraphs:  []string{full},
		FullText:    full,
		ContentType: "text/csv",
	}, nil
}

func (d *DocumentService) parseXLSX(path string) (*ParsedDocument, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	sheets := f.GetSheetList()
	out := &ParsedDocument{Kind: "xlsx", Sheets: sheets, ContentType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"}
	var all []string
	for _, s := range sheets {
		rows, err := f.GetRows(s)
		if err != nil || len(rows) == 0 {
			continue
		}
		tbl := make([][]string, 0, len(rows))
		for _, r := range rows {
			tbl = append(tbl, r)
		}
		out.Tables = append(out.Tables, tbl)
		all = append(all, "--- Sheet: "+s+" ---", renderTableAsText(tbl))
	}
	out.FullText = strings.Join(all, "\n\n")
	return out, nil
}

func (d *DocumentService) parseXLS(path string) (*ParsedDocument, error) {
	wb, err := xls.Open(path, "utf-8")
	if err != nil {
		return nil, err
	}
	sheets := make([]string, wb.NumSheets())
	out := &ParsedDocument{Kind: "xls", ContentType: "application/vnd.ms-excel"}
	var all []string
	for i := 0; i < wb.NumSheets(); i++ {
		sheet := wb.GetSheet(i)
		if sheet == nil {
			continue
		}
		sheets[i] = sheet.Name
		var rows [][]string
		for rowIdx := 0; rowIdx <= int(sheet.MaxRow); rowIdx++ {
			r := sheet.Row(rowIdx)
			if r == nil {
				continue
			}
			var cols []string
			for colIdx := 0; colIdx < int(r.LastCol()); colIdx++ {
				cols = append(cols, r.Col(colIdx))
			}
			rows = append(rows, cols)
		}
		if len(rows) > 0 {
			out.Tables = append(out.Tables, rows)
			all = append(all, "--- Sheet: "+sheet.Name+" ---", renderTableAsText(rows))
		}
	}
	out.Sheets = sheets
	out.FullText = strings.Join(all, "\n\n")
	return out, nil
}

func (d *DocumentService) parseDOCX(path string) (*ParsedDocument, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	doc, errP := docx.Parse(bytes.NewReader(raw), int64(len(raw)))
	if errP == nil {
		var b strings.Builder
		var paras []string
		for _, it := range doc.Document.Body.Items {
			switch para := it.(type) {
			case *docx.Paragraph:
				line := para.String()
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				paras = append(paras, line)
				b.WriteString(line)
				b.WriteString("\n")
			}
		}
		return &ParsedDocument{
			Kind:        "docx",
			Paragraphs:  paras,
			FullText:    b.String(),
			ContentType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		}, nil
	}
	// Fallback: just extract any readable text from inside the XML streams.
	return &ParsedDocument{
		Kind:        "docx",
		FullText:    string(raw)[:minInt(4000, len(raw))],
		ContentType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	}, nil
}

func (d *DocumentService) parsePDF(path string) (*ParsedDocument, error) {
	f, r, err := pdf.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	out := &ParsedDocument{Kind: "pdf", Pages: r.NumPage(), ContentType: "application/pdf"}
	var b strings.Builder
	var paras []string
	for i := 1; i <= r.NumPage(); i++ {
		p := r.Page(i)
		if p.V.IsNull() {
			continue
		}
		txt, err := p.GetPlainText(nil)
		if err != nil {
			continue
		}
		txt = strings.TrimSpace(txt)
		if txt == "" {
			continue
		}
		paras = append(paras, splitParagraphs(txt)...)
		b.WriteString(txt)
		b.WriteString("\n\n")
	}
	out.Paragraphs = paras
	out.FullText = b.String()
	return out, nil
}

// ----------------- helpers -----------------

func splitParagraphs(s string) []string {
	raw := strings.Split(s, "\n")
	out := make([]string, 0, len(raw))
	for _, p := range raw {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}

func renderTableAsText(rows [][]string) string {
	var b strings.Builder
	for i, r := range rows {
		if i > 2000 {
			b.WriteString("\n[... rows truncated ...]\n")
			break
		}
		for j := range r {
			r[j] = strings.ReplaceAll(r[j], "\n", " ")
			if len(r[j]) > 120 {
				r[j] = r[j][:120] + "…"
			}
		}
		b.WriteString(strings.Join(r, " \t "))
		b.WriteString("\n")
	}
	return b.String()
}

func sortedKeys(m map[string]any) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

func stringifyJSON(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case bool:
		if x {
			return "true"
		}
		return "false"
	case float64:
		return strings.TrimSuffix(strings.TrimSuffix(fmt.Sprintf("%.4f", x), "0"), ".")
	default:
		b, _ := json.Marshal(x)
		return string(b)
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// compile-time assertion that the reader interfaces we use exist.
var _ = io.Reader(nil)
