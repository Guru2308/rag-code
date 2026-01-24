package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Guru2308/rag-code/internal/errors"
)

// OllamaLLM implements the LLM client for Ollama
type OllamaLLM struct {
	baseURL string
	model   string
	client  *http.Client
}

// NewOllamaLLM creates a new Ollama LLM client
func NewOllamaLLM(baseURL, model string) *OllamaLLM {
	return &OllamaLLM{
		baseURL: baseURL,
		model:   model,
		client: &http.Client{
			// Generation can take longer
			Timeout: 2 * time.Minute,
		},
	}
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
	Stream   bool          `json:"stream"`
}

type ChatResponse struct {
	Message ChatMessage `json:"message"`
	Done    bool        `json:"done"`
}

// Generate generates a response for a given prompt (using Chat API for better context handling)
func (l *OllamaLLM) Generate(ctx context.Context, messages []ChatMessage) (string, error) {
	reqBody := ChatRequest{
		Model:    l.model,
		Messages: messages,
		Stream:   false,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", errors.Wrap(err, errors.ErrorTypeInternal, "failed to marshal request")
	}

	url := fmt.Sprintf("%s/api/chat", l.baseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", errors.Wrap(err, errors.ErrorTypeInternal, "failed to create request")
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := l.client.Do(req)
	if err != nil {
		return "", errors.Wrap(err, errors.ErrorTypeExternal, "failed to send request to Ollama")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", errors.New(errors.ErrorTypeExternal, fmt.Sprintf("Ollama returned non-200 status: %d, body: %s", resp.StatusCode, string(body)))
	}

	var res ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", errors.Wrap(err, errors.ErrorTypeInternal, "failed to decode response")
	}

	return res.Message.Content, nil
}

// StreamGenerate handles streaming responses
func (l *OllamaLLM) StreamGenerate(ctx context.Context, messages []ChatMessage, callback func(string) error) error {
	reqBody := ChatRequest{
		Model:    l.model,
		Messages: messages,
		Stream:   true,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return errors.Wrap(err, errors.ErrorTypeInternal, "failed to marshal request")
	}

	url := fmt.Sprintf("%s/api/chat", l.baseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return errors.Wrap(err, errors.ErrorTypeInternal, "failed to create request")
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := l.client.Do(req)
	if err != nil {
		return errors.Wrap(err, errors.ErrorTypeExternal, "failed to send request to Ollama")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errors.New(errors.ErrorTypeExternal, fmt.Sprintf("Ollama returned non-200 status: %d", resp.StatusCode))
	}

	decoder := json.NewDecoder(resp.Body)
	for {
		var res ChatResponse
		if err := decoder.Decode(&res); err != nil {
			if err == io.EOF {
				break
			}
			return errors.Wrap(err, errors.ErrorTypeInternal, "failed to decode stream")
		}

		if err := callback(res.Message.Content); err != nil {
			return err
		}

		if res.Done {
			break
		}
	}

	return nil
}
