// mcp-check is a remote client probe. It uses Streamable HTTP and reads the
// integration token from the environment; it never prints the credential.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type bearerTransport struct{ token string }

func (t bearerTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	copy := request.Clone(request.Context())
	copy.Header = request.Header.Clone()
	copy.Header.Set("Authorization", "Bearer "+t.token)
	return http.DefaultTransport.RoundTrip(copy)
}

func main() {
	endpoint := flag.String("url", "https://150.5.143.32/mcp", "Streamable HTTP endpoint")
	employeeID := flag.Int("employee", 0, "Optionally verify a cloud employee projection")
	flag.Parse()
	if err := check(*endpoint, *employeeID); err != nil {
		fmt.Fprintln(os.Stderr, "MCP verification failed:", err)
		os.Exit(1)
	}
}

func check(endpoint string, employeeID int) error {
	token := os.Getenv("CC4_MCP_TOKEN")
	if token == "" {
		return fmt.Errorf("CC4_MCP_TOKEN is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client := mcp.NewClient(&mcp.Implementation{Name: "cc4-remote-agent-check", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint: endpoint, HTTPClient: &http.Client{Transport: bearerTransport{token: token}}, MaxRetries: -1,
	}, nil)
	if err != nil {
		return err
	}
	defer session.Close()
	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		return err
	}
	want := map[string]bool{"salary_preview": true, "commission_preview": true, "level_transition_check": true, "employee_get": true, "workday_get": true, "performance_submit": true, "performance_approve": true}
	var names []string
	for _, tool := range tools.Tools {
		if !want[tool.Name] {
			return fmt.Errorf("unexpected tool %q", tool.Name)
		}
		names = append(names, tool.Name)
		delete(want, tool.Name)
	}
	if len(want) != 0 {
		return fmt.Errorf("expected tools are missing")
	}
	call := func(name string, arguments map[string]any) (map[string]any, error) {
		result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: arguments})
		if err != nil {
			return nil, err
		}
		if result.IsError {
			return nil, fmt.Errorf("tool %s returned an error", name)
		}
		raw, err := json.Marshal(result.StructuredContent)
		if err != nil {
			return nil, err
		}
		var object map[string]any
		err = json.Unmarshal(raw, &object)
		return object, err
	}
	commission, err := call("commission_preview", map[string]any{"dailyOrders": []int{3, 5, 8}})
	if err != nil || commission["monthlyTotal"] != float64(260) {
		return fmt.Errorf("commission preview mismatch: %v", err)
	}
	workdays, err := call("workday_get", map[string]any{"year": 2026, "month": 10})
	if err != nil || workdays["year"] != float64(2026) {
		return fmt.Errorf("workday query mismatch: %v", err)
	}
	if employeeID > 0 {
		employee, err := call("employee_get", map[string]any{"employeeId": employeeID})
		if err != nil || employee["id"] != float64(employeeID) {
			return fmt.Errorf("employee query mismatch: %v", err)
		}
	}
	// Another request after tools/call catches regressions in session retention.
	if _, err := session.ListTools(ctx, nil); err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]any{"status": "ok", "endpoint": endpoint, "tools": names, "commissionTotal": 260, "employeeVerified": employeeID > 0})
}
