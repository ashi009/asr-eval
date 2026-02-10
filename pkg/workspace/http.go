package workspace

import (
	"encoding/json"
	"net/http"
	"strings"
)

func (s *Service) RegisterRoutes(mux *http.ServeMux) {
	// Standard Methods
	mux.HandleFunc("GET /api/cases", s.handleListCases)
	mux.HandleFunc("GET /api/cases/{id}", s.handleGetCase)
	// Custom Methods - dispatched via POST /api/cases/{id} because {id}:suffix is not supported by ServeMux
	mux.HandleFunc("POST /api/cases/{id}", s.handleUpdateCaseOps)

	// Config
	mux.HandleFunc("GET /api/config", s.handleGetConfig)
}

func (s *Service) handleListCases(w http.ResponseWriter, r *http.Request) {
	cases, err := s.ListCases(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cases)
}

// handleGetCase handles GET /api/cases/{id}
func (s *Service) handleGetCase(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "ID required", http.StatusBadRequest)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	c, err := s.GetCase(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(c)
}

// handleUpdateCaseOps dispatches custom POST methods
func (s *Service) handleUpdateCaseOps(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	id, op, _ := strings.Cut(id, ":")
	r.SetPathValue("id", id)

	switch op {
	case "evaluate":
		s.handleEvaluateCase(w, r)
	case "generateContext":
		s.handleGenerateContext(w, r)
	case "updateContext":
		s.handleUpdateContext(w, r)
	default:
		http.Error(w, "Unknown method", http.StatusNotFound)
	}
}

// handleEvaluateCase handles POST /api/cases/{id}:evaluate
func (s *Service) handleEvaluateCase(w http.ResponseWriter, r *http.Request) {
	var req EvaluateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	req.ID = r.PathValue("id")

	report, err := s.Evaluate(r.Context(), req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(report)
}

// handleGenerateContext handles POST /api/cases/{id}:generateContext
func (s *Service) handleGenerateContext(w http.ResponseWriter, r *http.Request) {
	var req GenerateContextRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	req.ID = r.PathValue("id")

	ctx, err := s.GenerateContext(r.Context(), req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ctx)
}

// handleUpdateContext handles POST /api/cases/{id}:updateContext
func (s *Service) handleUpdateContext(w http.ResponseWriter, r *http.Request) {
	var req UpdateContextRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	req.ID = r.PathValue("id")

	updated, err := s.UpdateContext(r.Context(), req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(updated)
}

func (s *Service) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(Config{
		GenModel:         s.Config.GenModel,
		EvalModel:        s.Config.EvalModel,
		EnabledProviders: s.Config.EnabledProviders,
	})
}
