package server

import "net/http"

// handleSkillsList returns the names of all loaded skills as JSON.
func (s *Server) handleSkillsList(w http.ResponseWriter, r *http.Request) {
	if s.skillStore == nil {
		writeJSON(w, http.StatusOK, []any{})
		return
	}
	type skillInfo struct {
		Name string `json:"name"`
	}
	var result []skillInfo
	for _, name := range s.skillStore.Names() {
		result = append(result, skillInfo{Name: name})
	}
	if result == nil {
		result = []skillInfo{}
	}
	writeJSON(w, http.StatusOK, result)
}
