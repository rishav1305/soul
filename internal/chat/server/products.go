package server

import "net/http"

// productInfo is the response format for /api/products.
type productInfo struct {
	Name    string `json:"name"`
	Label   string `json:"label"`
	Tools   int    `json:"tools"`
	Running bool   `json:"running"`
	Icon    string `json:"icon,omitempty"`
}

// registeredProducts returns the list of products available in the product rail.
// This matches the product contexts defined in internal/chat/context/.
func registeredProducts() []productInfo {
	return []productInfo{
		{Name: "soul", Label: "Soul", Tools: 8, Running: true, Icon: "brain"},
		{Name: "tasks", Label: "Tasks", Tools: 6, Running: true, Icon: "list"},
		{Name: "tutor", Label: "Tutor", Tools: 7, Running: true, Icon: "graduation-cap"},
		{Name: "projects", Label: "Projects", Tools: 6, Running: true, Icon: "folder"},
		{Name: "observe", Label: "Observe", Tools: 4, Running: true, Icon: "activity"},
		{Name: "scout", Label: "Scout", Tools: 55, Running: true, Icon: "radar"},
		{Name: "sentinel", Label: "Sentinel", Tools: 7, Running: true, Icon: "shield"},
		{Name: "bench", Label: "Bench", Tools: 4, Running: true, Icon: "bar-chart"},
		{Name: "mesh", Label: "Mesh", Tools: 4, Running: true, Icon: "network"},
		{Name: "compliance", Label: "Compliance", Tools: 4, Running: true, Icon: "check-circle"},
		{Name: "devops", Label: "DevOps", Tools: 2, Running: true, Icon: "terminal"},
		{Name: "dataeng", Label: "DataEng", Tools: 2, Running: true, Icon: "database"},
		{Name: "docs", Label: "Docs", Tools: 2, Running: true, Icon: "file-text"},
	}
}

// handleProducts returns the list of available products.
func (s *Server) handleProducts(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, registeredProducts())
}
