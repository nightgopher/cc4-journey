package mcp

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nightgopher/cc4-journey/app/internal/access"
	"github.com/nightgopher/cc4-journey/app/internal/mcpaccess"
)

type grantResolver func(context.Context, int64) (*mcpaccess.Grant, error)

func toolDenied(message string) *mcp.CallToolResult {
	return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: message}}}
}

func installAuthorization(server *mcp.Server, fallbackID int64, resolve grantResolver) {
	server.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, request mcp.Request) (mcp.Result, error) {
			if method != "tools/list" && method != "tools/call" {
				return next(ctx, method, request)
			}
			id := fallbackID
			if extra := request.GetExtra(); extra != nil && extra.TokenInfo != nil {
				id, _ = extra.TokenInfo.Extra["credentialId"].(int64)
			}
			if id <= 0 {
				return nil, access.ErrUnauthenticated
			}
			// Resolve again for every message: role, team, modules and revocation
			// are not a snapshot attached to the initialized MCP session.
			grant, err := resolve(ctx, id)
			if err != nil {
				return nil, access.ErrUnauthenticated
			}
			ctx = mcpaccess.WithGrant(ctx, grant)
			if method == "tools/list" {
				result, err := next(ctx, method, request)
				if err != nil {
					return nil, err
				}
				list, ok := result.(*mcp.ListToolsResult)
				if !ok {
					return nil, fmt.Errorf("invalid tools/list result")
				}
				filtered := make([]*mcp.Tool, 0, len(list.Tools))
				for _, tool := range list.Tools {
					if grant.CanTool(ctx, tool.Name) {
						filtered = append(filtered, tool)
					}
				}
				list.Tools = filtered
				mcpaccess.Audit(ctx, grant, "tools_list", nil, "success")
				return list, nil
			}
			params, ok := request.GetParams().(*mcp.CallToolParamsRaw)
			if !ok {
				return nil, fmt.Errorf("invalid tools/call parameters")
			}
			if !grant.CanTool(ctx, params.Name) {
				mcpaccess.Audit(ctx, grant, params.Name, params.Arguments, "denied")
				return toolDenied("该接入凭据或账号无权调用此工具"), nil
			}
			if err := grant.CheckTarget(ctx, params.Name, params.Arguments); err != nil {
				mcpaccess.Audit(ctx, grant, params.Name, params.Arguments, "denied")
				return toolDenied("无权访问指定的业务对象或参数无效"), nil
			}
			result, err := next(ctx, method, request)
			outcome := "success"
			if call, ok := result.(*mcp.CallToolResult); err != nil || (ok && call.IsError) {
				outcome = "error"
			}
			mcpaccess.Audit(ctx, grant, params.Name, params.Arguments, outcome)
			return result, err
		}
	})
}
