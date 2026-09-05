package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"scm/internal/memory"
	"scm/internal/model"
	"scm/pkg/authx"
	"scm/pkg/llmclient"
)

// assistantMaxIters caps the function-calling loop so a confused model cannot
// spin forever.
const assistantMaxIters = 6

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
		System:      "你是 SCM 供应链管理系统的管理员助手，具备全部模块权限。优先用查询工具解析物料/供应商/产品 ID 后再执行创建或下单；只调用你被提供的工具；回答用中文，简洁准确。",
	},
	{
		Role: model.RoleProcSpec, Name: "采购专员助手",
		Description: "采购下单、采购单与物料/供应商/库存查询",
		System:      "你是采购专员助手，负责采购下单与采购相关查询。创建采购单前先查询供应商（须 APPROVED）和物料 ID；只调用你被提供的工具；回答用中文，简洁准确。",
	},
	{
		Role: model.RoleProcMgr, Name: "采购经理助手",
		Description: "物料与供应商准入维护、采购审批",
		System:      "你是采购经理助手，负责物料/供应商主数据维护与采购管控。优先用查询工具解析 ID 后再创建或更新；只调用你被提供的工具；回答用中文，简洁准确。",
	},
	{
		Role: model.RolePlanSpec, Name: "计划专员助手",
		Description: "需求与 MRP 计划、库存/物料查询",
		System:      "你是计划专员助手，负责需求与 MRP 计划相关查询。优先用查询工具解析物料/产品 ID；只调用你被提供的工具；回答用中文，简洁准确。",
	},
	{
		Role: model.RolePlanSup, Name: "计划主管助手",
		Description: "BOM 管理、计划发布与物料/产品查询",
		System:      "你是计划主管助手，负责 BOM 创建维护与计划发布。创建 BOM 前先查询产品 ID 与组件物料 ID；只调用你被提供的工具；回答用中文，简洁准确。",
	},
	{
		Role: model.RoleQC, Name: "质检员助手",
		Description: "收货质检相关查询",
		System:      "你是质检员助手，负责收货质检相关查询。优先用查询工具解析采购单/物料 ID；只调用你被提供的工具；回答用中文，简洁准确。",
	},
	{
		Role: model.RoleWhMgr, Name: "仓库管理员助手",
		Description: "仓储与出入库、库存查询",
		System:      "你是仓库管理员助手，负责仓储与库存查询。优先用查询工具解析物料/仓库/库位 ID；只调用你被提供的工具；回答用中文，简洁准确。",
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
	Reply     string   `json:"reply"`
	Agent     string   `json:"agent"`
	AgentName string   `json:"agent_name"`
	ToolCalls []string `json:"tool_calls"`
}

// Chat is the assistant entry point. The user's identity (actor) is threaded
// through so every tool can authorize against it.
func (s *AssistantService) Chat(ctx context.Context, actor *authx.Actor, message string) (*AssistantReply, error) {
	if actor == nil || actor.TenantID == 0 {
		return nil, errf(ErrUnauthorized, "login required")
	}
	if strings.TrimSpace(message) == "" {
		return nil, errorsBadRequest("message is required")
	}
	agent, err := s.route(ctx, actor, message)
	if err != nil {
		return nil, err
	}
	return s.run(ctx, actor, agent, message)
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
