package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"github.com/RoboConOxfordshire/shepherd/internal/config"
	"github.com/RoboConOxfordshire/shepherd/internal/editor"
	"github.com/RoboConOxfordshire/shepherd/internal/gpio"
	"github.com/RoboConOxfordshire/shepherd/internal/run"
	"github.com/RoboConOxfordshire/shepherd/internal/upload"
	"github.com/RoboConOxfordshire/shepherd/internal/ws"
)

type Server struct {
	cfg        *config.Config
	mux        http.Handler
	runMgr     *run.Manager
	editorH    *editor.Handler
	uploadH    *upload.UploadHandler
	wsHub      *ws.Hub
	camStream  *ws.CameraStreamer
	logStream  *ws.LogStreamer
	pylsProxy  *ws.PylsProxy
	gpioBtn    *gpio.Button
	staticFS   fs.FS
	upgrader   websocket.Upgrader
}

func New(cfg *config.Config, staticFS fs.FS) (*Server, error) {
	runCfg := run.Config{
		UserCodeEntrypoint: cfg.UserCodeEntrypoint,
		UserCodePath:       cfg.UserCodePath,
		RobotLibPath:       cfg.RobotLibPath,
		OutputFilePath:     cfg.OutputFilePath,
		RoundLength:        cfg.RoundLength,
		ReapGracePeriod:    cfg.ReapGracePeriod,
	}

	runMgr, err := run.NewManager(runCfg)
	if err != nil {
		return nil, fmt.Errorf("run manager: %w", err)
	}

	wsHub := ws.NewHub()

	s := &Server{
		cfg:       cfg,
		runMgr:    runMgr,
		wsHub:     wsHub,
		staticFS:  staticFS,
		upgrader:  websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }},
	}
	s.editorH = editor.NewHandler(filepath.Join("robotsrc"))
	s.uploadH = upload.NewHandler(cfg.UserCodePath, cfg.UserCodeEntrypoint, s.onUpload)
	s.camStream = ws.NewCameraStreamer(wsHub, cfg.CameraImagePath, cfg.ImageWidth, cfg.ImageHeight)
	s.logStream = ws.NewLogStreamer(wsHub, cfg.OutputFilePath)
	s.pylsProxy = ws.NewPylsProxy(cfg.RobotLibPath)

	s.gpioBtn = gpio.NewButton(cfg.StartButtonPin, cfg.ArenaUSBPath, s.onStartButton)
	s.setupRoutes()

	return s, nil
}

func (s *Server) onUpload() {
	s.runMgr.Stop()
	s.runMgr.Reset()
	if err := s.runMgr.StartUserCode(); err != nil {
		log.Printf("restart user code after upload: %v", err)
	}
}

func (s *Server) onStartButton(zone string) {
	if s.runMgr.State() == run.StateReady {
		s.runMgr.StartUserCode()
		s.runMgr.SendStartSignal(zone, run.ModeCompetition)
		s.runMgr.StartReaper()
	}
}

func (s *Server) Run(ctx context.Context) error {
	s.camStream.Start()
	s.logStream.Start()
	s.gpioBtn.Start()

	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", s.cfg.Port),
		Handler: s.mux,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		httpServer.Shutdown(shutdownCtx)
		s.runMgr.Cleanup()
	}()

	log.Printf("Shepherd listening on :%d", s.cfg.Port)
	if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}
	return nil
}

func (s *Server) setupRoutes() {
	mux := http.NewServeMux()

	// Run API
	mux.HandleFunc("GET /api/run/status", s.handleRunStatus)
	mux.HandleFunc("POST /api/run/start", s.handleRunStart)
	mux.HandleFunc("POST /api/run/stop", s.handleRunStop)
	mux.HandleFunc("GET /api/run/output", s.handleRunOutput)
	mux.HandleFunc("GET /api/run/time-left", s.handleRunTimeLeft)
	mux.HandleFunc("GET /api/run/picture", s.handleRunPicture)

	// Upload API
	mux.HandleFunc("GET /api/upload", s.handleUploadIndex)
	mux.HandleFunc("POST /api/upload", s.handleUploadFile)

	// Editor API
	mux.HandleFunc("GET /api/files", s.handleEditorList)
	mux.HandleFunc("GET /api/files/{filename}", s.handleEditorRead)
	mux.HandleFunc("POST /api/files/{filename}", s.handleEditorWrite)
	mux.HandleFunc("DELETE /api/files/{filename}", s.handleEditorDelete)

	// WebSocket endpoints
	mux.HandleFunc("GET /ws/camera", s.handleWSCamera)
	mux.HandleFunc("GET /ws/logs", s.handleWSLogs)
	mux.HandleFunc("GET /ws/pyls", s.handleWSPyls)

	// Static files
	mux.HandleFunc("GET /", s.handleStatic)

	s.mux = corsMiddleware(mux)
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// --- Run handlers ---

func (s *Server) handleRunStatus(w http.ResponseWriter, r *http.Request) {
	state := s.runMgr.State()
	writeJSON(w, map[string]any{
		"state": state.String(),
		"zone":  s.runMgr.Zone(),
		"mode":  s.runMgr.Mode(),
	})
}

func (s *Server) handleRunStart(w http.ResponseWriter, r *http.Request) {
	if s.runMgr.State() != run.StateReady {
		http.Error(w, "not ready", http.StatusConflict)
		return
	}

	zone := r.FormValue("zone")
	modeStr := r.FormValue("mode")
	if zone == "" {
		zone = "0"
	}
	mode := run.ModeDevelopment
	if modeStr == "competition" {
		mode = run.ModeCompetition
	}

	if err := s.runMgr.StartUserCode(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := s.runMgr.SendStartSignal(zone, mode); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if mode == run.ModeCompetition {
		s.runMgr.StartReaper()
	}

	writeJSON(w, map[string]string{"status": "started"})
}

func (s *Server) handleRunStop(w http.ResponseWriter, r *http.Request) {
	s.runMgr.Stop()
	writeJSON(w, map[string]string{"status": "stopped"})
}

func (s *Server) handleRunOutput(w http.ResponseWriter, r *http.Request) {
	data, err := os.ReadFile(s.cfg.OutputFilePath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write(data)
}

func (s *Server) handleRunTimeLeft(w http.ResponseWriter, r *http.Request) {
	left := s.runMgr.TimeLeft()
	writeJSON(w, map[string]any{"time_left": left})
}

func (s *Server) handleRunPicture(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, s.cfg.CameraImagePath)
}

// --- Upload handlers ---

func (s *Server) handleUploadIndex(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]string{"message": "Send a POST with a .py or .zip file to /api/upload"})
}

func (s *Server) handleUploadFile(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<20)

	file, header, err := r.FormFile("uploaded_file")
	if err != nil {
		http.Error(w, "missing file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	if err := s.uploadH.ProcessUpload(file, header); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, map[string]string{"status": "ok"})
}

// --- Editor handlers ---

func (s *Server) handleEditorList(w http.ResponseWriter, r *http.Request) {
	list, err := s.editorH.ListFiles()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, list)
}

func (s *Server) handleEditorRead(w http.ResponseWriter, r *http.Request) {
	filename := r.PathValue("filename")
	content, err := s.editorH.ReadFile(filename)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, map[string]string{"filename": filename, "content": content})
}

func (s *Server) handleEditorWrite(w http.ResponseWriter, r *http.Request) {
	filename := r.PathValue("filename")
	data, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := s.editorH.WriteFile(filename, string(data)); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]string{"status": "saved"})
}

func (s *Server) handleEditorDelete(w http.ResponseWriter, r *http.Request) {
	filename := r.PathValue("filename")
	if err := s.editorH.DeleteFile(filename); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]string{"status": "deleted"})
}

// --- WebSocket handlers ---

func (s *Server) handleWSCamera(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	frame := s.camStream.GetCurrentFrame()
	if frame != "" {
		conn.WriteMessage(websocket.TextMessage, []byte("[CAMERA]"+frame))
	}

	s.wsHub.Add(conn)
	defer s.wsHub.Remove(conn)

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

func (s *Server) handleWSLogs(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.wsHub.Add(conn)
	defer s.wsHub.Remove(conn)

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

func (s *Server) handleWSPyls(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.pylsProxy.Handle(conn)
}

// --- Static file handler ---

func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	if path == "/" {
		path = "/editor/index.html"
	}

	// Check if path without trailing slash is a directory
	checkPath := strings.TrimSuffix(path, "/")
	if s.staticFS != nil {
		if f, err := s.staticFS.Open(strings.TrimPrefix(checkPath, "/")); err == nil {
			if stat, err := f.Stat(); err == nil && stat.IsDir() {
				f.Close()
				if !strings.HasSuffix(path, "/") {
					http.Redirect(w, r, path+"/", http.StatusMovedPermanently)
					return
				}
				path = path + "index.html"
			} else {
				f.Close()
			}
		}
	}

	path = strings.TrimPrefix(path, "/")

	if s.staticFS != nil {
		data, err := fs.ReadFile(s.staticFS, path)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		http.ServeContent(w, r, path, time.Time{}, bytes.NewReader(data))
		return
	}

	http.NotFound(w, r)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}


