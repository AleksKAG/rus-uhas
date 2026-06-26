package rest

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"rus-uhas/internal/domain"
	"rus-uhas/internal/hal"
	"rus-uhas/internal/telemetry"
)

// Server - REST API сервер для UI
type Server struct {
	router          *chi.Mux
	protocolManager *domain.ProtocolManager
	generator       hal.Generator
	logger          *telemetry.Logger
	auth            *AuthService
	wsHub           *WebSocketHub
}

// Config - конфигурация REST сервера
type Config struct {
	Port            int
	CORSOrigins     []string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
}

// NewServer создает новый REST сервер
func NewServer(
	cfg Config,
	pm *domain.ProtocolManager,
	gen hal.Generator,
	logger *telemetry.Logger,
	auth *AuthService,
	wsHub *WebSocketHub,
) *Server {
	s := &Server{
		router:          chi.NewRouter(),
		protocolManager: pm,
		generator:       gen,
		logger:          logger,
		auth:            auth,
		wsHub:           wsHub,
	}
	
	s.setupMiddleware(cfg)
	s.setupRoutes()
	
	return s
}

// setupMiddleware настраивает middleware
func (s *Server) setupMiddleware(cfg Config) {
	// Recovery от паник
	s.router.Use(middleware.Recoverer)
	
	// Request ID для трейсинга
	s.router.Use(middleware.RequestID)
	
	// Real IP для корректного логирования
	s.router.Use(middleware.RealIP)
	
	// Structured logging
	s.router.Use(s.loggingMiddleware)
	
	// Timeouts
	s.router.Use(middleware.Timeout(30 * time.Second))
	
	// CORS
	s.router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.CORSOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))
}

// setupRoutes настраивает маршруты
func (s *Server) setupRoutes() {
	// Публичные endpoints (без аутентификации)
	s.router.Get("/health", s.healthHandler)
	s.router.Get("/ready", s.readyHandler)
	s.router.Post("/api/v1/auth/login", s.loginHandler)
	
	// Защищенные endpoints
	s.router.Route("/api/v1", func(r chi.Router) {
		r.Use(s.authMiddleware)
		
		// Протоколы
		r.Get("/protocols", s.listProtocolsHandler)
		r.Get("/protocols/active", s.getActiveProtocolHandler)
		r.Put("/protocols/active", s.setActiveProtocolHandler)
		
		// Состояние системы
		r.Get("/state", s.getSystemStateHandler)
		
		// Операции
		r.Post("/operations", s.startOperationHandler)
		r.Delete("/operations/current", s.stopOperationHandler)
		r.Get("/operations/history", s.getOperationHistoryHandler)
		
		// Алерты
		r.Get("/alerts", s.getActiveAlertsHandler)
		r.Post("/alerts/{id}/acknowledge", s.acknowledgeAlertHandler)
		
		// Настройки
		r.Get("/settings", s.getSettingsHandler)
		r.Put("/settings", s.updateSettingsHandler)
		
		// Пользователь
		r.Get("/user", s.getCurrentUserHandler)
		r.Post("/auth/logout", s.logoutHandler)
	})
	
	// WebSocket endpoint
	s.router.Get("/ws", s.wsHandler)
}

// Run запускает HTTP сервер
func (s *Server) Run(cfg Config) error {
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      s.router,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}
	
	s.logger.Info("Запуск REST API сервера", "port", cfg.Port)
	return srv.ListenAndServe()
}

// Shutdown gracefully останавливает сервер
func (s *Server) Shutdown(ctx context.Context) error {
	return s.router.Shutdown(ctx)
}

// loggingMiddleware логирует все запросы
func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		
		defer func() {
			s.logger.WithContext(r.Context()).Info("HTTP request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", ww.Status(),
				"duration_ms", time.Since(start).Milliseconds(),
				"bytes", ww.BytesWritten(),
				"remote_addr", r.RemoteAddr)
		}()
		
		next.ServeHTTP(ww, r)
	})
}

// === Handlers ===

func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) readyHandler(w http.ResponseWriter, r *http.Request) {
	// Проверяем, что генератор доступен
	ctx := r.Context()
	_, err := s.generator.GetState(ctx)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"status": "not_ready",
			"error":  err.Error(),
		})
		return
	}
	
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (s *Server) listProtocolsHandler(w http.ResponseWriter, r *http.Request) {
	protocols := []ProtocolResponse{
		{
			ID:          string(domain.ProtocolNeuro),
			Name:        "Нейрохирургия",
			Description: "Высокая точность, низкая мощность. Для работы с мозгом и нервами.",
			Tissues:     []string{"soft", "vessel", "nerve", "tumor"},
		},
		{
			ID:          string(domain.ProtocolHepatic),
			Name:        "Гепатология",
			Description: "Быстрая диссекция печени с сохранением сосудов.",
			Tissues:     []string{"soft", "vessel"},
		},
		{
			ID:          string(domain.ProtocolWound),
			Name:        "Хирургия ран",
			Description: "Импульсный режим для деликатной очистки ран.",
			Tissues:     []string{"soft"},
		},
	}
	
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"protocols": protocols,
	})
}

func (s *Server) getActiveProtocolHandler(w http.ResponseWriter, r *http.Request) {
	protocol := s.protocolManager.GetCurrentProtocol()
	if protocol == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{
			"error": "no active protocol",
		})
		return
	}
	
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"protocol": ProtocolResponse{
			ID:   protocol.Name(),
			Name: protocol.Name(),
		},
	})
}

func (s *Server) setActiveProtocolHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ProtocolID string `json:"protocol_id"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid request body",
		})
		return
	}
	
	protocolType := domain.ProtocolType(req.ProtocolID)
	if err := s.protocolManager.SetProtocol(protocolType); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
		return
	}
	
	s.logger.WithContext(r.Context()).Info("Протокол изменен через API",
		"protocol", req.ProtocolID,
		"user", telemetry.GetSurgeonID(r.Context()))
	
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
	})
}

func (s *Server) getSystemStateHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	state, err := s.generator.GetState(ctx)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
		return
	}
	
	protocol := s.protocolManager.GetCurrentProtocol()
	protocolName := ""
	if protocol != nil {
		protocolName = protocol.Name()
	}
	
	user := getCurrentUserFromContext(ctx)
	
	writeJSON(w, http.StatusOK, SystemStateResponse{
		Generator: GeneratorStateResponse{
			IsFiring:       state.IsFiring,
			PowerWatts:     state.PowerWatts,
			FrequencyHz:    state.FrequencyHz,
			TipTempC:       state.TipTempC,
			ImpedanceOhms:  state.ImpedanceOhms,
			AspirationBar:  state.AspirationBar,
			IrrigationMl:   state.IrrigationMl,
