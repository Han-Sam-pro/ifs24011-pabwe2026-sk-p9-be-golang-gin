package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Message adalah satu pesan chat untuk OpenAI Chat Completions.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Completer adalah kontrak client LLM.
type Completer interface {
	Complete(ctx context.Context, messages []Message) (string, error)
}

// OpenAIClient adalah HTTP client untuk endpoint /chat/completions.
type OpenAIClient struct {
	Endpoint string
	Key      string
	Model    string
	HTTP     *http.Client
}

// NewOpenAIClient membuat client OpenAI-compatible dengan timeout default.
func NewOpenAIClient(endpoint, key, model string) *OpenAIClient {
	return &OpenAIClient{Endpoint: endpoint, Key: key, Model: model, HTTP: &http.Client{Timeout: 120 * time.Second}}
}

type chatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature"`
}

type chatResponse struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Complete mengirim percakapan ke model dan mengembalikan isi jawaban.
func (c *OpenAIClient) Complete(ctx context.Context, messages []Message) (string, error) {
	payload, _ := json.Marshal(chatRequest{Model: c.Model, Messages: messages, Temperature: 0.7})
	url := strings.TrimRight(c.Endpoint, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.Key)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var out chatResponse
	decodeErr := json.NewDecoder(resp.Body).Decode(&out)
	if resp.StatusCode >= http.StatusMultipleChoices {
		msg := http.StatusText(resp.StatusCode)
		if out.Error != nil && out.Error.Message != "" {
			msg = out.Error.Message
		}
		return "", fmt.Errorf("openai mengembalikan status %d: %s", resp.StatusCode, msg)
	}
	if decodeErr != nil {
		return "", fmt.Errorf("respons openai tidak valid: %w", decodeErr)
	}
	if len(out.Choices) == 0 || strings.TrimSpace(out.Choices[0].Message.Content) == "" {
		return "", errors.New("respons openai tidak berisi jawaban")
	}
	return out.Choices[0].Message.Content, nil
}
