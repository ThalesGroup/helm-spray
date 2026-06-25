package web

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// Server is the web GUI server
type Server struct {
	Addr      string
	ChartDir  string
	Namespace string
	hub       *Hub
}

// NewServer creates a new web server
func NewServer(addr, chartDir, namespace string) *Server {
	hub := NewHub()
	go hub.Run()

	return &Server{
		Addr:      addr,
		ChartDir:  chartDir,
		Namespace: namespace,
		hub:       hub,
	}
}

// Start starts the web server
func (s *Server) Start() error {
	mux := http.NewServeMux()

	// Static files
	fs := http.FileServer(http.Dir("internal/web/static"))
	mux.Handle("/static/", http.StripPrefix("/static/", fs))

	// API endpoints
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/api/charts", s.handleCharts)
	mux.HandleFunc("/api/chart", s.handleChart)
	mux.HandleFunc("/api/releases", s.handleReleases)
	mux.HandleFunc("/api/spray", s.handleSpray)
	mux.HandleFunc("/ws", s.handleWebSocket)

	fmt.Printf("Helm Spray Web GUI listening on %s\n", s.Addr)
	return http.ListenAndServe(s.Addr, mux)
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, "internal/web/static/index.html")
}

func (s *Server) handleCharts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	charts, err := ScanCharts(s.ChartDir)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{"charts": charts}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleChart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	chartName := r.URL.Query().Get("name")
	if chartName == "" {
		http.Error(w, "Missing chart name", http.StatusBadRequest)
		return
	}

	info, err := GetChartInfo(s.ChartDir, chartName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(info); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleReleases(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	releases, err := GetReleases(s.Namespace)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(releases); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleSpray(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req SprayRequest
	if err := readJSON(r, &req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Start spray in background, logs will be streamed via WebSocket
	go s.runSpray(req)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "started"}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) runSpray(req SprayRequest) {
	// Build command args
	args := []string{"spray", req.ChartName, "--namespace", req.Namespace}

	if req.Debug {
		args = append(args, "--debug")
	}
	if req.Verbose {
		args = append(args, "--verbose")
	}
	if req.DryRun {
		args = append(args, "--dry-run")
	}
	if req.Force {
		args = append(args, "--force")
	}
	if req.CreateNamespace {
		args = append(args, "--create-namespace")
	}
	if req.ResetValues {
		args = append(args, "--reset-values")
	}
	if req.ReuseValues {
		args = append(args, "--reuse-values")
	}
	if req.Timeout > 0 {
		args = append(args, "--timeout", fmt.Sprintf("%d", req.Timeout))
	}
	if req.PrefixReleases != "" {
		args = append(args, "--prefix-releases", req.PrefixReleases)
	}
	for _, f := range req.ValueFiles {
		args = append(args, "-f", f)
	}
	for _, v := range req.Values {
		args = append(args, "--set", v)
	}
	for _, t := range req.Targets {
		args = append(args, "--target", t)
	}
	for _, x := range req.Excludes {
		args = append(args, "--exclude", x)
	}

	// Broadcast start
	s.hub.Broadcast([]byte(fmt.Sprintf("[spray] Starting spray for chart: %s\n", req.ChartName)))

	// Execute helm spray command
	output, err := ExecCommand("helm", args...)
	if err != nil {
		s.hub.Broadcast([]byte(fmt.Sprintf("[spray] Error: %s\n", err.Error())))
		s.hub.Broadcast([]byte(fmt.Sprintf("[spray] Output:\n%s\n", output)))
		return
	}

	s.hub.Broadcast([]byte(fmt.Sprintf("[spray] Output:\n%s\n", output)))
	s.hub.Broadcast([]byte("[spray] Spray completed successfully\n"))
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	ServeWs(s.hub, w, r)
}
