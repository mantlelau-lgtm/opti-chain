# [OPEN] 调试：智能助手报错 "抱歉，处理步骤过多，请简化问题后重试"

## 会话元信息
- session id: `assistant-max-iters`
- 创建时间: 2026-09-10
- 现象: 偶发触发助手的兜底回复 "抱歉，处理步骤过多，请简化问题后重试"，对应 assistant.go 里 `assistantMaxIters = 6` 循环耗尽

## 触发条件（已知）
```
assistantMaxIters = 6
runMultimodal for i:=0; i<6; i++ {
    resp := llm.Chat(...)
    if choice.FinishReason == "tool_calls" && len(tool_calls)>0 → 继续
    else → return reply
}
→ 循环结束仍未返回 → 触发兜底回复
```

## 可证伪假设 H1 ~ H5

| # | 假设 | 预期观测 | 可观测点 |
|---|---|---|---|
| H1 | **模型每次都返回 `finish_reason=tool_calls` + 真实 tool calls，但这 6+ 次都在 "A 工具调用后触发 B，B 触发 C…" 级联，模型没有收敛** | 6 轮每轮都有 tool_calls，且 tools 各不相同或循环，最后一次仍不是 stop | 每轮 i、finish_reason、len(tool_calls)、每个 tc.Function.Name、choice.Message.Content 非空前缀 |
| H2 | **模型实际已经回答完毕（有 content），但 finish_reason 仍是 "tool_calls" / "length" 等非 stop 值** | 最后几轮 `choice.Message.Content` 已经有完整回答，但循环继续 | 每轮 content 前 200 字节 + finish_reason |
| H3 | **tool_calls 里的某个工具参数解析失败（error JSON 回写 tool role），导致模型反复重试同一个工具** | 连续 2+ 轮都叫同一个 tool name，且 executeTool 返回 JSON 含 error 字段 | tc.Function.Name 序列 + `s.executeTool` 返回内容（含 error 前缀时） |
| H4 | **模型把 system prompt 里的"可调用工具"当成必答题，每轮都生成工具调用**（system prompt 里的工具描述引导有问题） | 即便只是闲聊，也会触发 N 次无意义工具调用 | 实际问一句话不含任何查询类语句，也要 tool_calls |
| H5 | **`choice.FinishReason` / `resp.Choices` 字段被网关（llm-gw / Ollama mlx）返回了空或非标准值**（例如返回 `finish_reason=stop` 但同时含 tool_calls；或 choices 为空；或 `finish_reason = ""`）| 第 N 轮 choice 为空数组，或 finish_reason=""，或同时有 content 和 tool_calls 但 finish_reason=tool_calls 让我们走了 tool 分支忽略 content | `choice.FinishReason` 原始值、`len(resp.Choices)`、choice.Message.ToolCalls 非空但 content 也非空 |

## 计划
1. Step 2: 静态阅读 registerAssistantTools，看是否存在循环调用 schema（A 结果 → 触发 B，B 结果 → 触发 A）
2. Step 3: 在 runMultimodal 的 for 循环内加 zap 调试日志（不改业务逻辑，仅打印）
3. Step 4: 让用户触发一次报错，分析日志验证假设
4. Step 5: 做最小修复（调大 maxIters？/ 有 content 就返回？/ 错误处理收敛？/ 限制同名工具连续调用？）
