// Command scm-mcp is the Model Context Protocol server for SCM. It exposes the
// ordering and base-data capabilities as MCP tools over stdio, so a local
// agent (e.g. via the local llm-gw) can discover and call them.
//
// Configuration (environment variables):
//
//	SCM_MCP_BASE_URL  SCM backend base URL       (default http://127.0.0.1:8088)
//	SCM_MCP_AK        Access Key                 (required)
//	SCM_MCP_SK        Secret Key                 (required)
//
// The AK/SK is issued by a tenant administrator in the SCM web UI
// (系统 → API 密钥) and scopes the agent to that tenant + a permission set.
package main

import (
	"log"
	"os"

	"github.com/mark3labs/mcp-go/server"
)

func main() {
	baseURL := getEnv("SCM_MCP_BASE_URL", "http://127.0.0.1:8088")
	ak := os.Getenv("SCM_MCP_AK")
	sk := os.Getenv("SCM_MCP_SK")
	if ak == "" || sk == "" {
		log.Fatal("SCM_MCP_AK and SCM_MCP_SK are required — issue a key in the SCM 系统 → API 密钥 page")
	}

	client := NewClient(baseURL, ak, sk)

	s := server.NewMCPServer("scm-mcp", "0.1.0", server.WithToolCapabilities(true))
	registerTools(s, client)

	if err := server.ServeStdio(s); err != nil {
		log.Fatalf("stdio server: %v", err)
	}
}

func getEnv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
