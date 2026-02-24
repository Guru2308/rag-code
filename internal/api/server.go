package api

import (
	"context"
	"net/http"
	"time"

	"github.com/Guru2308/rag-code/internal/domain"
	"github.com/Guru2308/rag-code/internal/indexing"
	"github.com/Guru2308/rag-code/internal/llm"
	"github.com/Guru2308/rag-code/internal/logger"
	"github.com/Guru2308/rag-code/internal/prompt"
	"github.com/Guru2308/rag-code/internal/retrieval"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// Server handles HTTP requests
type Server struct {
	Router    *gin.Engine
	indexer   *indexing.Indexer
	retriever *retrieval.Retriever
	llm       *llm.OllamaLLM
	prompter  prompt.Generator
	port      string
}

// NewServer creates a new API server
func NewServer(port string, indexer *indexing.Indexer, retriever *retrieval.Retriever, llmClient *llm.OllamaLLM, prompter prompt.Generator) *Server {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())

	// Add simple logging middleware
	router.Use(func(c *gin.Context) {
		start := time.Now()
		c.Next()
		logger.Debug("Inbound request",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"duration", time.Since(start),
		)
	})

	s := &Server{
		Router:    router,
		indexer:   indexer,
		retriever: retriever,
		llm:       llmClient,
		prompter:  prompter,
		port:      port,
	}

	s.setupRoutes()
	return s
}

func (s *Server) setupRoutes() {
	// Swagger documentation
	s.Router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	api := s.Router.Group("/api")
	{
		api.POST("/index", s.handleIndex)
		api.POST("/query", s.handleQuery)
		api.GET("/status", s.handleStatus)
	}
}

// Start runs the HTTP server (blocks until shutdown)
func (s *Server) Start() error {
	logger.Info("Starting API server", "port", s.port)
	return s.Router.Run(":" + s.port)
}

// ListenAndServe runs the HTTP server and returns when shutdown is requested via ctx.
// Use this for graceful shutdown on SIGINT/SIGTERM.
func (s *Server) ListenAndServe(ctx context.Context) error {
	srv := &http.Server{
		Addr:    ":" + s.port,
		Handler: s.Router,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP server error", "error", err)
		}
	}()

	<-ctx.Done()
	logger.Info("Shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}
	logger.Info("Server stopped")
	return nil
}

type indexRequest struct {
	Path string `json:"path" binding:"required"`
}

// handleIndex starts indexing a project
// @Summary      Index a codebase
// @Description  Recursively parse and index code files from the given path
// @Tags         indexing
// @Accept       json
// @Produce      json
// @Param        request  body      indexRequest  true  "Path to index"
// @Success      202      {object}  map[string]string
// @Failure      400      {object}  map[string]string
// @Router       /index [post]
func (s *Server) handleIndex(c *gin.Context) {
	var req indexRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		if err := s.indexer.Index(ctx, req.Path); err != nil {
			logger.Error("Background indexing failed", "path", req.Path, "error", err)
		}
	}()

	c.JSON(http.StatusAccepted, gin.H{"status": "indexing_started", "path": req.Path})
}

// handleQuery handles codebase queries
// @Summary      Query the codebase
// @Description  Search and answer questions about the codebase using hybrid retrieval and LLM
// @Tags         query
// @Accept       json
// @Produce      json
// @Param        query  body      domain.SearchQuery  true  "Search query"
// @Success      200    {object}  map[string]interface{}
// @Failure      400    {object}  map[string]string
// @Failure      500    {object}  map[string]string
// @Router       /query [post]
func (s *Server) handleQuery(c *gin.Context) {
	var req domain.SearchQuery
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.MaxResults == 0 {
		req.MaxResults = 5
	}

	// 1. Retrieve relevant chunks
	results, err := s.retriever.Retrieve(c.Request.Context(), req)
	if err != nil {
		logger.Error("Retrieval failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve context"})
		return
	}

	// 2. Prepare LLM prompt using the prompter
	promptStr, err := s.prompter.Generate(c.Request.Context(), req.Query, results)
	if err != nil {
		logger.Error("Prompt generation failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate prompt"})
		return
	}

	messages := []llm.ChatMessage{
		{
			Role:    "user",
			Content: promptStr,
		},
	}

	// 3. Generate response
	response, err := s.llm.Generate(c.Request.Context(), messages)
	if err != nil {
		logger.Error("LLM generation failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate response"})
		return
	}

	logger.Info("Generated LLM response", "query", req.Query, "response_length", len(response))

	c.JSON(http.StatusOK, gin.H{
		"response": response,
		"results":  results,
	})
}

// handleStatus returns the server status
// @Summary      Health check
// @Description  Check if the API server is alive
// @Tags         system
// @Produce      json
// @Success      200  {object}  map[string]string
// @Router       /status [get]
func (s *Server) handleStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "alive"})
}
