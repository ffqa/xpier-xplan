package server

import (
	"net/http"
	"os"

	"github.com/ffqa/xpier-xplan/internal/mcp"
)

type mcpToolView struct {
	Name        string `json:"name"`
	Title       string `json:"title"`
	Description string `json:"description"`
	ReadOnly    bool   `json:"read_only"`
}

// handleMCPInfo tells the settings page how agents connect: the endpoint URL,
// the binary for the stdio bridge, and the tools they will see.
func (s *Server) handleMCPInfo(w http.ResponseWriter, r *http.Request) {
	binary, err := os.Executable()
	if err != nil {
		s.logger.Printf("mcp info: locate executable: %v", err)
		binary = "xplan"
	}
	tools := []mcpToolView{}
	for _, tool := range s.mcp.Tools() {
		tools = append(tools, mcpToolView{Name: tool.Name, Title: tool.Title, Description: tool.Description, ReadOnly: tool.ReadOnly()})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"url":    s.app.BaseURL() + mcp.Path,
		"binary": binary,
		"tools":  tools,
	})
}
