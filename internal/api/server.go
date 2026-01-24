package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/Guru2308/rag-code/internal/domain"
	"github.com/Guru2308/rag-code/internal/indexing"
	"github.com/Guru2308/rag-code/internal/llm"
	"github.com/Guru2308/rag-code/internal/logger"
	"github.com/Guru2308/rag-code/internal/retrieval"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// Server handles HTTP requests
type Server struct {
	router    *gin.Engine
	indexer   *indexing.Indexer
	retriever *retrieval.Retriever
	llm       *llm.OllamaLLM
	port      string
}

// NewServer creates a new API server
func NewServer(port string, indexer *indexing.Indexer, retriever *retrieval.Retriever, llmClient *llm.OllamaLLM) *Server {
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
		router:    router,
		indexer:   indexer,
		retriever: retriever,
		llm:       llmClient,
		port:      port,
	}

	s.setupRoutes()
	return s
}

func (s *Server) setupRoutes() {
	// Swagger documentation
	s.router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	api := s.router.Group("/api")
	{
		api.POST("/index", s.handleIndex)
		api.POST("/query", s.handleQuery)
		api.GET("/status", s.handleStatus)
	}
}

// Start runs the HTTP server
func (s *Server) Start() error {
	logger.Info("Starting API server", "port", s.port)
	return s.router.Run(":" + s.port)
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

	// 2. Prepare LLM prompt
	messages := []llm.ChatMessage{
		{
			Role:    "system",
			Content: "You are a helpful code assistant. Use the provided code context to answer the user's question.",
		},
	}

	contextStr := "Code Context:\n"
	for _, res := range results {
		contextStr += fmt.Sprintf("\n--- %s (Lines %d-%d) ---\n%s\n",
			res.Chunk.FilePath, res.Chunk.StartLine, res.Chunk.EndLine, res.Chunk.Content)
	}

	messages = append(messages, llm.ChatMessage{
		Role:    "user",
		Content: fmt.Sprintf("%s\n\nQuestion: %s", contextStr, req.Query),
	})

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
