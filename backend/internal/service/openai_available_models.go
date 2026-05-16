package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/httpclient"
	openaipkg "github.com/Wei-Shaw/sub2api/internal/pkg/openai"
)

var ErrOpenAIModelListUnsupported = errors.New("openai upstream model list requires an OAuth access token or API key")

type openAIModelsResponse struct {
	Data []openaiModelListItem `json:"data"`
}

type openaiModelListItem struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

func (s *AccountTestService) FetchOpenAIUpstreamModels(ctx context.Context, account *Account) ([]openaipkg.Model, error) {
	if account == nil {
		return nil, errors.New("account is nil")
	}
	if account.Platform != PlatformOpenAI {
		return nil, fmt.Errorf("not an openai account")
	}

	token, baseURL, err := s.resolveOpenAIModelListAuth(ctx, account)
	if err != nil {
		return nil, err
	}
	return requestOpenAIAvailableModels(ctx, account, baseURL, token)
}

func (s *AccountTestService) resolveOpenAIModelListAuth(ctx context.Context, account *Account) (token, baseURL string, err error) {
	if account.IsOpenAIOAuth() {
		if s == nil || s.openAITokenProvider == nil {
			token = strings.TrimSpace(account.GetOpenAIAccessToken())
		} else {
			token, err = s.openAITokenProvider.GetAccessToken(ctx, account)
			if err != nil {
				return "", "", fmt.Errorf("get openai access token failed: %w", err)
			}
		}
		if strings.TrimSpace(token) == "" {
			return "", "", ErrOpenAIModelListUnsupported
		}
		return token, "https://api.openai.com", nil
	}

	if account.IsOpenAIApiKey() {
		token = strings.TrimSpace(account.GetOpenAIApiKey())
		if token == "" {
			return "", "", ErrOpenAIModelListUnsupported
		}
		baseURL = account.GetOpenAIBaseURL()
		if baseURL == "" {
			baseURL = "https://api.openai.com"
		}
		if s != nil {
			normalized, err := s.validateUpstreamBaseURL(baseURL)
			if err != nil {
				return "", "", fmt.Errorf("invalid base_url: %w", err)
			}
			baseURL = normalized
		}
		return token, baseURL, nil
	}

	return "", "", ErrOpenAIModelListUnsupported
}

func requestOpenAIAvailableModels(ctx context.Context, account *Account, baseURL, token string) ([]openaipkg.Model, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = "https://api.openai.com"
	}
	modelsURL := baseURL
	if !strings.HasSuffix(modelsURL, "/models") {
		if strings.HasSuffix(modelsURL, "/v1") {
			modelsURL += "/models"
		} else {
			modelsURL += "/v1/models"
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, modelsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create openai model list request failed: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(token))
	if userAgent := strings.TrimSpace(account.GetOpenAIUserAgent()); userAgent != "" {
		req.Header.Set("User-Agent", userAgent)
	}

	client, err := httpclient.GetClient(httpclient.Options{
		ProxyURL:           accountProxyURL(account),
		Timeout:            30 * time.Second,
		ValidateResolvedIP: true,
		AllowPrivateHosts:  isLoopbackEndpoint(modelsURL),
	})
	if err != nil {
		return nil, fmt.Errorf("create openai model list client failed: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openai model list request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read openai model list response failed: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &kiroUsageHTTPError{StatusCode: resp.StatusCode, Body: strings.TrimSpace(string(body))}
	}

	var parsed openAIModelsResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("decode openai model list response failed: %w", err)
	}

	models := make([]openaipkg.Model, 0, len(parsed.Data))
	for _, item := range parsed.Data {
		id := strings.TrimSpace(item.ID)
		if id == "" {
			continue
		}
		models = append(models, openaipkg.Model{
			ID:          id,
			Object:      firstNonEmptyOpenAIModelField(item.Object, "model"),
			Created:     item.Created,
			OwnedBy:     item.OwnedBy,
			Type:        "model",
			DisplayName: id,
		})
	}
	return models, nil
}

func firstNonEmptyOpenAIModelField(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
