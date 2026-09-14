package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"scm/internal/memory"
	"scm/internal/model"
	"scm/pkg/authx"
	"scm/pkg/llmclient"

	"go.uber.org/zap"
)

// Attachment describes a user-uploaded file carried into the chat. It matches
// the mirror struct in handler so both packages can exchange attachments
// without a circular dependency.
type Attachment struct {
	ID       string
	Name     string
	MIME     string
	StoredAt string
	Size     int64
}

// assistantMaxIters caps the function-calling loop so a confused model cannot
// spin forever. Default is 12; composite tasks (document parse → logistics
// route → material create → po_create + tool re-confirm + final answer) can
// easily exceed the original 6.
const assistantMaxIters = 12

// AssistantDeps holds the service collaborators the assistant tools call.
type AssistantDeps struct {
	Audit     *AuditService
	Memory    *memory.Service
	Materials *MaterialService
	Suppliers *SupplierService
	Products  *ProductService
	BOMs      *BOMService
	POs       *PurchaseOrderService
	Stock     *StockService
	RBAC      *RBACService
	SOs       *SalesOrderService
	Customers *CustomerService
	Logistics *LogisticsService
	Docs      *DocumentService
}

// AssistantTool is one operation exposed to the LLM. Perm gates the tool
// against the caller's actual permissions at execution time.
type AssistantTool struct {
	Name        string
	Description string
	Perm        string
	Schema      map[string]any
	Exec        func(actor *authx.Actor, args map[string]any) (any, error)
}

// agentMeta is the static personality of one role-agent. The tool set is
// derived dynamically from the role's permission matrix.
type agentMeta struct {
	Role        string
	Name        string
	Description string // used by the router to pick among a user's roles
	System      string // system prompt
}

// agentMetas defines one agent per role (7 fixed roles). System prompts keep
// the model honest: prefer query tools to resolve IDs, only call tools it is
// given, and answer in Chinese.
var agentMetas = []agentMeta{
	{
		Role: model.RoleAdmin, Name: "管理员助手",
		Description: "全流程管理：采购、物料、BOM、计划、审批等所有供应链操作",
		System:      "你是 SCM 供应链管理系统的管理员助手，具备全部模块权限。优先用查询工具解析物料/供应商/产品 ID 后再执行创建或下单；用户要求「批量创建/批量导入/批量写入 N 条数据」时，优先调用对应 xxx_batch_create 批量工具（如 material_batch_create），一次性提交 items 数组（单事务保证要么全成功要么全失败），绝对不要「先查 xxx_list 翻整本目录→再循环 xxx_create 逐条写入」的低效率模式，那样会消耗大量 token 并超出处理步数。只调用你被提供的工具；回答用中文，简洁准确。",
	},
	{
		Role: model.RoleProcSpec, Name: "采购专员助手",
		Description: "采购下单、采购单与物料/供应商/库存查询",
		System:      "你是采购专员助手，负责采购下单与采购相关查询。创建采购单前先查询供应商（须 APPROVED）和物料 ID；批量创建多张 PO 或批量导入物料/供应商时，优先调用对应 batch 批量工具，一次性落库（单事务），不要翻列表再逐条写。只调用你被提供的工具；回答用中文，简洁准确。",
	},
	{
		Role: model.RoleProcMgr, Name: "采购经理助手",
		Description: "物料与供应商准入维护、采购审批",
		System:      "你是采购经理助手，负责物料/供应商主数据维护与采购管控。优先用查询工具解析 ID 后再创建或更新；批量导入物料/供应商时，直接用 material_batch_create 或供应商批量工具（单事务要么全成要么全败），不要翻列表再逐条写。只调用你被提供的工具；回答用中文，简洁准确。",
	},
	{
		Role: model.RolePlanSpec, Name: "计划专员助手",
		Description: "需求与 MRP 计划、库存/物料查询",
		System:      "你是计划专员助手，负责需求与 MRP 计划相关查询。优先用查询工具解析物料/产品 ID；批量场景优先用对应 batch 工具一次性写入（单事务），不要翻列表。只调用你被提供的工具；回答用中文，简洁准确。",
	},
	{
		Role: model.RolePlanSup, Name: "计划主管助手",
		Description: "BOM 管理、计划发布与物料/产品查询",
		System:      "你是计划主管助手，负责 BOM 创建维护与计划发布。创建 BOM 前先查询产品 ID 与组件物料 ID；批量维护产品/物料主档时，优先调用 material_batch_create 等批量工具（单事务保证一致性），不要翻列表逐条写。只调用你被提供的工具；回答用中文，简洁准确。",
	},
	{
		Role: model.RoleQC, Name: "质检员助手",
		Description: "收货质检相关查询",
		System:      "你是质检员助手，负责收货质检相关查询。优先用查询工具解析采购单/物料 ID；如果出现批量写的场景（用户提的），一律用 batch 工具，不要翻列表逐条写。只调用你被提供的工具；回答用中文，简洁准确。",
	},
	{
		Role: model.RoleWhMgr, Name: "仓库管理员助手",
		Description: "仓储与出入库、库存查询",
		System:      "你是仓库管理员助手，负责仓储与库存查询。优先用查询工具解析物料/仓库 ID；批量入库/批量调整场景优先调用对应 batch 工具（单事务），不要逐页翻库存列表。只调用你被提供的工具；回答用中文，简洁准确。",
	},
}

// AssistantService routes a user question to the role-appropriate agent and
// runs its function-calling loop against the internal services.
type AssistantService struct {
	llm   *llmclient.Client
	deps  AssistantDeps
	tools []*AssistantTool
}

func NewAssistantService(llm *llmclient.Client, deps AssistantDeps) *AssistantService {
	s := &AssistantService{llm: llm, deps: deps}
	s.tools = registerAssistantTools(deps)
	return s
}

// AssistantReply is what the chat endpoint returns.
type AssistantReply struct {
	Reply     string         `json:"reply"`
	Agent     string         `json:"agent"`
	AgentName string         `json:"agent_name"`
	ToolCalls []string       `json:"tool_calls"`
	Usage     AssistantUsage `json:"usage"`
}

// AssistantUsage carries per-request telemetry shown to the user in the UI.
type AssistantUsage struct {
	Model            string `json:"model"`
	TotalMs          int64  `json:"total_ms"`
	PromptTokens     int    `json:"prompt_tokens"`
	CompletionTokens int    `json:"completion_tokens"`
	TotalTokens      int    `json:"total_tokens"`
	LLMCalls         int    `json:"llm_calls"`
}

// Chat is the assistant entry point. The user's identity (actor) is threaded
// through so every tool can authorize against it.
func (s *AssistantService) Chat(ctx context.Context, actor *authx.Actor, message string) (*AssistantReply, error) {
	return s.ChatWithAttachments(ctx, actor, message, nil)
}

// ChatWithAttachments accepts user text plus uploaded attachments (text or
// images). Text files are inlined as a `---- FILE: <name> ----` block; images
// are encoded as base64 data URIs and rendered as image_url parts in the
// multi-modal user message.
func (s *AssistantService) ChatWithAttachments(ctx context.Context, actor *authx.Actor, message string, attachments []Attachment) (*AssistantReply, error) {
	if actor == nil || actor.TenantID == 0 {
		return nil, errf(ErrUnauthorized, "login required")
	}
	if strings.TrimSpace(message) == "" && len(attachments) == 0 {
		return nil, errorsBadRequest("message or attachment is required")
	}
	textAttachments, imageAttachments, err := s.classifyAttachments(attachments)
	if err != nil {
		return nil, err
	}
	userText := s.composeUserText(message, textAttachments)
	routeText := userText
	if len(routeText) > 2000 {
		routeText = routeText[:2000]
	}
	agent, err := s.route(ctx, actor, routeText)
	if err != nil {
		return nil, err
	}
	return s.runMultimodal(ctx, actor, agent, userText, imageAttachments)
}

func (s *AssistantService) classifyAttachments(attachments []Attachment) (texts []Attachment, images []Attachment, err error) {
	const maxTextBytes = 2 << 20 // 2 MB 文档上限（内部 DocumentService 还有 2MB 安全兜底）
	const maxImageBytes = 5 << 20
	docExts := map[string]bool{
		".pdf": true, ".docx": true, ".xlsx": true, ".xls": true,
		".txt": true, ".md": true, ".markdown": true, ".csv": true, ".tsv": true, ".json": true, ".log": true,
	}
	imageExts := map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".webp": true, ".gif": true, ".bmp": true}
	for _, a := range attachments {
		mime := strings.ToLower(a.MIME)
		ext := strings.ToLower(filepath.Ext(a.Name))
		isImage := strings.HasPrefix(mime, "image/") || imageExts[ext]
		isDoc := docExts[ext] || strings.HasPrefix(mime, "text/") ||
			mime == "application/json" || mime == "application/pdf" ||
			strings.Contains(mime, "spreadsheet") || strings.Contains(mime, "wordprocessing")
		switch {
		case isImage:
			if a.Size > maxImageBytes {
				return nil, nil, errorsBadRequest(fmt.Sprintf("image %s exceeds 5 MB", a.Name))
			}
			images = append(images, a)
		case isDoc:
			if a.Size > maxTextBytes {
				return nil, nil, errorsBadRequest(fmt.Sprintf("document %s exceeds 2 MB", a.Name))
			}
			texts = append(texts, a)
		default:
			return nil, nil, errorsBadRequest(fmt.Sprintf("unsupported attachment type: %s (%s)", a.Name, mime))
		}
	}
	return
}

func (s *AssistantService) composeUserText(message string, texts []Attachment) string {
	var b strings.Builder
	b.WriteString(strings.TrimSpace(message))
	for _, t := range texts {
		var content string
		if s.deps.Docs != nil {
			parsed, err := s.deps.Docs.ParseFile(context.Background(), t.StoredAt, t.Name)
			if err == nil {
				content = s.deps.Docs.Summary(parsed, 12000) + "\n\n=== 全文片段（前 8k 字符）===\n" + parsed.Preview
			} else {
				content = fmt.Sprintf("<parse error: %v>", err)
			}
		} else {
			raw, err := os.ReadFile(t.StoredAt)
			if err == nil {
				content = string(raw)
			} else {
				content = fmt.Sprintf("<read error: %v>", err)
			}
		}
		if len(content) > 25000 {
			content = content[:25000] + "\n...[已截断，原文件内容过长]..."
		}
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		fmt.Fprintf(&b, "---- 文件: %s (%s, %d 字节) ----\n%s\n---- END FILE ----",
			t.Name, t.MIME, t.Size, content)
	}
	return b.String()
}

func (s *AssistantService) runMultimodal(ctx context.Context, actor *authx.Actor, agent *agentMeta, userText string, images []Attachment) (*AssistantReply, error) {
	start := time.Now()
	usage := AssistantUsage{Model: s.llm.Model()}

	roleActor := &authx.Actor{Roles: []string{agent.Role}}
	tools := s.toolsFor(roleActor)

	// === Memory injection ===================================================
	// 1) Pull long-term (profile + top facts) and short-term (recent turns)
	//    from the memory module if it is wired in. The long-term layer is
	//    appended to the system prompt; the short-term turns are inserted
	//    between system and the fresh user message so the model sees the
	//    full conversation in chronological order.
	var (
		longProfile   string
		longFacts     []string
		shortTermMsgs []llmclient.Message
	)
	if s.deps.Memory != nil {
		if r := s.deps.Memory.Retrieve(ctx, actor); r != nil {
			longProfile = r.LongTerm.Profile
			longFacts = append(longFacts, r.LongTerm.Facts...)
			for _, e := range r.ShortTerm {
				if e.Role == "assistant" {
					shortTermMsgs = append(shortTermMsgs, llmclient.Message{
						Role:    "assistant",
						Content: e.Content,
					})
				} else {
					shortTermMsgs = append(shortTermMsgs, llmclient.Message{
						Role:    "user",
						Content: e.Content,
					})
				}
			}
		}
	}
	systemContent := agent.System
	if longProfile != "" || len(longFacts) > 0 {
		var b strings.Builder
		b.WriteString(systemContent)
		b.WriteString("\n\n== 长期记忆（用户画像 + 历史偏好）==\n")
		if longProfile != "" {
			b.WriteString("画像: " + longProfile + "\n")
		}
		if len(longFacts) > 0 {
			b.WriteString("偏好事实:\n")
			for _, f := range longFacts {
				b.WriteString("  - " + f + "\n")
			}
		}
		b.WriteString("== END 长期记忆 ==")
		systemContent = b.String()
	}

	msgs := []llmclient.Message{
		{Role: "system", Content: systemContent},
	}
	msgs = append(msgs, shortTermMsgs...)
	// === end Memory injection ===============================================

	// Build the user message: pure text if no images, otherwise content parts.
	if len(images) == 0 {
		msgs = append(msgs, llmclient.Message{Role: "user", Content: userText})
	} else {
		parts := []llmclient.ContentPart{{Type: "text", Text: userText}}
		for _, img := range images {
			dataURI, err := s.encodeImage(img)
			if err != nil {
				return nil, err
			}
			parts = append(parts, llmclient.ContentPart{
				Type:     "image_url",
				ImageURL: &llmclient.ImageURL{URL: dataURI, Detail: "auto"},
			})
		}
		msgs = append(msgs, llmclient.Message{Role: "user", ContentList: parts})
	}

	llmTools := make([]llmclient.Tool, 0, len(tools))
	for _, t := range tools {
		llmTools = append(llmTools, llmclient.Tool{
			Type:     "function",
			Function: llmclient.Function{Name: t.Name, Description: t.Description, Parameters: t.Schema},
		})
	}

	var invoked []string
	// recentToolCalls is a sliding window used to short-circuit a confused
	// model before it exhausts the full assistantMaxIters budget.
	// Trigger rules (any hits):
	//   - identical (name + arg-signature) repeated N=3 consecutively
	//   - same name with monotonically increasing page ("page-flip") for N=5
	//     times (only checked for list* tools that carry a page arg)
	type toolTrace struct {
		name string
		sig  string
		page int // 0 if the tool does not expose a page arg
	}
	const (
		maxRepeatExact = 3
		maxRepeatPage  = 5
	)
	recent := make([]toolTrace, 0, maxRepeatPage+4)
	argsSig := func(name, rawArgs string) (string, int) {
		var m map[string]any
		if err := json.Unmarshal([]byte(rawArgs), &m); err == nil {
			kw, _ := m["keyword"].(string)
			p, _ := m["page"].(float64)
			sz, _ := m["size"].(float64)
			sig := fmt.Sprintf("k=%q:p=%d:s=%d", kw, int(p), int(sz))
			delete(m, "keyword")
			delete(m, "page")
			delete(m, "size")
			if len(m) > 0 {
				sig += fmt.Sprintf(":extra=%v", m)
			}
			return name + "|" + sig, int(p)
		}
		return name + "|" + rawArgs, 0
	}
	blockedReply := func(reason string) *AssistantReply {
		usage.TotalMs = time.Since(start).Milliseconds()
		return &AssistantReply{
			Reply:     reason,
			Agent:     agent.Role,
			AgentName: agent.Name,
			ToolCalls: invoked,
			Usage:     usage,
		}
	}

	for i := 0; i < assistantMaxIters; i++ {
		resp, err := s.llm.Chat(ctx, msgs, llmTools)
		if err != nil {
			return nil, err
		}
		usage.LLMCalls++
		if resp.Model != "" {
			usage.Model = resp.Model
		}
		usage.PromptTokens += resp.Usage.PromptTokens
		usage.CompletionTokens += resp.Usage.CompletionTokens
		usage.TotalTokens += resp.Usage.TotalTokens
		if len(resp.Choices) == 0 {
			zap.L().Warn("[DBG assistant iters empty choices]",
				zap.Int("iter", i), zap.Int("prompt_tokens", resp.Usage.PromptTokens),
				zap.Int("completion_tokens", resp.Usage.CompletionTokens))
			usage.TotalMs = time.Since(start).Milliseconds()
			return &AssistantReply{
				Reply:     "抱歉，助手返回了空结果，请稍后重试。",
				Agent:     agent.Role,
				AgentName: agent.Name,
				ToolCalls: invoked,
				Usage:     usage,
			}, nil
		}
		choice := resp.Choices[0]
		content := choice.Message.Content
		contentPreview := content
		if len(contentPreview) > 200 {
			contentPreview = contentPreview[:200]
		}
		tcNames := make([]string, 0, len(choice.Message.ToolCalls))
		for _, tc := range choice.Message.ToolCalls {
			tcNames = append(tcNames, tc.Function.Name)
		}
		zap.L().Info("[DBG assistant iters]",
			zap.Int("iter", i),
			zap.String("agent_role", agent.Role),
			zap.String("finish_reason", choice.FinishReason),
			zap.Int("num_tool_calls", len(choice.Message.ToolCalls)),
			zap.Strings("tool_call_names", tcNames),
			zap.String("content_preview", contentPreview),
			zap.Int("prompt_tokens", resp.Usage.PromptTokens),
			zap.Int("completion_tokens", resp.Usage.CompletionTokens),
			zap.Int("total_tokens", resp.Usage.TotalTokens),
		)
		if choice.FinishReason == "tool_calls" && len(choice.Message.ToolCalls) > 0 {
			// --- short-circuit: check repeated tool patterns BEFORE executing ---
			for _, tc := range choice.Message.ToolCalls {
				sig, page := argsSig(tc.Function.Name, tc.Function.Arguments)
				recent = append(recent, toolTrace{name: tc.Function.Name, sig: sig, page: page})
				if len(recent) > maxRepeatPage+4 {
					recent = recent[len(recent)-(maxRepeatPage+4):]
				}
				// rule 1: exact (name+sig) repeated maxRepeatExact times at tail
				if len(recent) >= maxRepeatExact {
					tail := recent[len(recent)-maxRepeatExact:]
					allSame := true
					for _, t := range tail[1:] {
						if t.name != tail[0].name || t.sig != tail[0].sig {
							allSame = false
							break
						}
					}
					if allSame {
						zap.L().Warn("[DBG assistant tool loop BLOCKED exact]",
							zap.String("tool", tail[0].name),
							zap.String("sig", tail[0].sig),
							zap.Int("count", maxRepeatExact))
						return blockedReply(fmt.Sprintf(
							"已连续 %d 次调用 %s 且参数没有变化，没有收敛。请提供更精确的名称/编码（SKU、PO 号、供应商编号等）让我用 keyword 精搜；如果是要批量创建/写入数据，请直接把要写入的每一条内容发给我，我会用批量创建工具一次性写入（单事务保证要么全成功要么全失败）。",
							maxRepeatExact, tail[0].name,
						)), nil
					}
				}
				// rule 2: same tool with strictly increasing page (page-flip)
				if strings.HasPrefix(tc.Function.Name, "material_") ||
					strings.HasPrefix(tc.Function.Name, "supplier_") ||
					strings.HasPrefix(tc.Function.Name, "product_") ||
					strings.HasPrefix(tc.Function.Name, "po_") ||
					strings.HasPrefix(tc.Function.Name, "so_") ||
					strings.HasPrefix(tc.Function.Name, "stock_") ||
					strings.HasPrefix(tc.Function.Name, "customer_") {
					n := len(recent)
					if n >= maxRepeatPage {
						flip := recent[n-maxRepeatPage : n]
						// same name & strictly growing page & each has page>0
						ok := true
						prevPage := -1
						for _, t := range flip {
							if t.name != flip[0].name || t.page <= prevPage || t.page == 0 {
								ok = false
								break
							}
							prevPage = t.page
						}
						if ok {
							zap.L().Warn("[DBG assistant tool loop BLOCKED page-flip]",
								zap.String("tool", flip[0].name),
								zap.Int("start_page", flip[0].page),
								zap.Int("end_page", flip[len(flip)-1].page),
								zap.Int("count", maxRepeatPage))
							return blockedReply(fmt.Sprintf(
								"已连续翻 %d 页查询 %s 仍未收敛，这通常是因为你想批量写入数据但缺少精确的编码。请直接把要处理的每一条的名称/编码（SKU、PO、SO 号等）发给我；若是批量创建，我会调用批量创建工具一次性落库（单事务保证要么全成功要么全失败），不会再逐页翻列表。",
								maxRepeatPage, flip[0].name,
							)), nil
						}
					}
				}
			}
			// --- end short-circuit ---
			msgs = append(msgs, llmclient.Message{Role: "assistant", Content: choice.Message.Content, ToolCalls: choice.Message.ToolCalls})
			for _, tc := range choice.Message.ToolCalls {
				invoked = append(invoked, tc.Function.Name)
				msgs = append(msgs, llmclient.Message{
					Role:       "tool",
					ToolCallID: tc.ID,
					Name:       tc.Function.Name,
					Content:    s.executeTool(actor, tc),
				})
			}
			continue
		}
		usage.TotalMs = time.Since(start).Milliseconds()
		return &AssistantReply{
			Reply: choice.Message.Content, Agent: agent.Role, AgentName: agent.Name,
			ToolCalls: invoked, Usage: usage,
		}, nil
	}
	zap.L().Warn("[DBG assistant iters EXHAUSTED]",
		zap.Int("max_iters", assistantMaxIters),
		zap.String("agent_role", agent.Role),
		zap.Strings("invoked_tool_calls", invoked),
		zap.Int("llm_calls", usage.LLMCalls),
		zap.Int("prompt_tokens", usage.PromptTokens),
		zap.Int("completion_tokens", usage.CompletionTokens),
		zap.Int("total_tokens", usage.TotalTokens),
	)
	usage.TotalMs = time.Since(start).Milliseconds()
	// Robustness: when the loop exhausts, prefer to return any non-empty
	// assistant content emitted in the last round over the generic "too many
	// steps" fallback. This papers over models that set finish_reason != "stop"
	// even when the answer is fully written (H2 & H5 in debug md).
	lastContent := ""
	if len(msgs) > 0 {
		last := msgs[len(msgs)-1]
		if last.Role == "assistant" && strings.TrimSpace(last.Content) != "" {
			lastContent = last.Content
		}
	}
	if lastContent != "" {
		return &AssistantReply{
			Reply:     lastContent,
			Agent:     agent.Role,
			AgentName: agent.Name,
			ToolCalls: invoked,
			Usage:     usage,
		}, nil
	}
	return &AssistantReply{
		Reply:     "抱歉，处理步骤过多，请简化问题后重试。",
		Agent:     agent.Role,
		AgentName: agent.Name,
		ToolCalls: invoked,
		Usage:     usage,
	}, nil
}

func (s *AssistantService) encodeImage(img Attachment) (string, error) {
	raw, err := os.ReadFile(img.StoredAt)
	if err != nil {
		return "", fmt.Errorf("read image %s: %w", img.Name, err)
	}
	mime := img.MIME
	if mime == "" {
		switch strings.ToLower(filepath.Ext(img.Name)) {
		case ".png":
			mime = "image/png"
		case ".jpg", ".jpeg":
			mime = "image/jpeg"
		case ".webp":
			mime = "image/webp"
		case ".gif":
			mime = "image/gif"
		case ".bmp":
			mime = "image/bmp"
		default:
			mime = "image/png"
		}
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(raw), nil
}

// route wakes the agent for the user's question: single-role users map
// directly, multi-role users are classified by the LLM against the allowed
// agents' descriptions.
func (s *AssistantService) route(ctx context.Context, actor *authx.Actor, message string) (*agentMeta, error) {
	if len(actor.Roles) == 0 {
		return nil, errf(ErrForbidden, "current user has no role")
	}
	allowed := s.allowedAgents(actor.Roles)
	if len(allowed) == 1 {
		return &allowed[0], nil
	}
	role, err := s.classify(ctx, allowed, message)
	if err == nil {
		for i := range allowed {
			if allowed[i].Role == role {
				return &allowed[i], nil
			}
		}
	}
	// fall back to the first allowed role
	return &allowed[0], nil
}

func (s *AssistantService) allowedAgents(roles []string) []agentMeta {
	var out []agentMeta
	for _, m := range agentMetas {
		for _, r := range roles {
			if m.Role == r {
				out = append(out, m)
				break
			}
		}
	}
	return out
}

// classify asks the LLM to pick one role among the allowed agents.
func (s *AssistantService) classify(ctx context.Context, allowed []agentMeta, message string) (string, error) {
	var b strings.Builder
	b.WriteString("你是路由助手。根据用户问题，从以下助手中选择最合适的一个，只回复其角色代码，不要任何其它内容。\n")
	for _, a := range allowed {
		fmt.Fprintf(&b, "- %s：%s\n", a.Role, a.Description)
	}
	resp, err := s.llm.Chat(ctx, []llmclient.Message{
		{Role: "system", Content: b.String()},
		{Role: "user", Content: message},
	}, nil)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(resp.Choices[0].Message.Content), nil
}

// run executes the agent's function-calling loop. The tool set offered to the
// LLM is the ROLE's set; every actual execution re-checks the USER's permission.
func (s *AssistantService) run(ctx context.Context, actor *authx.Actor, agent *agentMeta, message string) (*AssistantReply, error) {
	roleActor := &authx.Actor{Roles: []string{agent.Role}}
	tools := s.toolsFor(roleActor)

	msgs := []llmclient.Message{
		{Role: "system", Content: agent.System},
		{Role: "user", Content: message},
	}
	llmTools := make([]llmclient.Tool, 0, len(tools))
	for _, t := range tools {
		llmTools = append(llmTools, llmclient.Tool{
			Type:     "function",
			Function: llmclient.Function{Name: t.Name, Description: t.Description, Parameters: t.Schema},
		})
	}

	var invoked []string
	for i := 0; i < assistantMaxIters; i++ {
		resp, err := s.llm.Chat(ctx, msgs, llmTools)
		if err != nil {
			return nil, err
		}
		choice := resp.Choices[0]
		if choice.FinishReason == "tool_calls" && len(choice.Message.ToolCalls) > 0 {
			msgs = append(msgs, llmclient.Message{Role: "assistant", Content: choice.Message.Content, ToolCalls: choice.Message.ToolCalls})
			for _, tc := range choice.Message.ToolCalls {
				invoked = append(invoked, tc.Function.Name)
				msgs = append(msgs, llmclient.Message{
					Role:       "tool",
					ToolCallID: tc.ID,
					Name:       tc.Function.Name,
					Content:    s.executeTool(actor, tc),
				})
			}
			continue
		}
		return &AssistantReply{Reply: choice.Message.Content, Agent: agent.Role, AgentName: agent.Name, ToolCalls: invoked}, nil
	}
	return &AssistantReply{
		Reply:     "抱歉，处理步骤过多，请简化问题后重试。",
		Agent:     agent.Role,
		AgentName: agent.Name,
		ToolCalls: invoked,
	}, nil
}

// toolsFor returns the tools the given actor is permitted to use.
func (s *AssistantService) toolsFor(a *authx.Actor) []*AssistantTool {
	var out []*AssistantTool
	for _, t := range s.tools {
		if s.deps.RBAC.HasPerm(a, t.Perm) {
			out = append(out, t)
		}
	}
	return out
}

// executeTool runs one tool call, carrying the user's identity so the tool can
// authorize internally against the caller's actual permissions.
func (s *AssistantService) executeTool(actor *authx.Actor, tc llmclient.ToolCall) string {
	t := s.findTool(tc.Function.Name)
	if t == nil {
		return `{"error":"未知工具 ` + tc.Function.Name + `"}`
	}
	if !s.deps.RBAC.HasPerm(actor, t.Perm) {
		return `{"error":"无权限执行 ` + t.Name + `（需要权限 ` + t.Perm + `）"}`
	}
	args := map[string]any{}
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
		return `{"error":"参数解析失败: ` + err.Error() + `"}`
	}
	result, err := t.Exec(actor, args)
	if err != nil {
		return `{"error":"` + err.Error() + `"}`
	}
	b, err := json.Marshal(result)
	if err != nil {
		return `{"error":"结果序列化失败"}`
	}
	return string(b)
}

func (s *AssistantService) findTool(name string) *AssistantTool {
	for _, t := range s.tools {
		if t.Name == name {
			return t
		}
	}
	return nil
}
