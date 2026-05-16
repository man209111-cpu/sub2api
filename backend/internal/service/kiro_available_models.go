package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/httpclient"
	kiropkg "github.com/Wei-Shaw/sub2api/internal/pkg/kiro"
	"github.com/google/uuid"
)

var ErrKiroModelListUnsupported = errors.New("kiro upstream model list requires an OAuth/access-token account")

type kiroAvailableModelsResponse struct {
	AvailableModels      []kiroAvailableModelItem `json:"availableModels"`
	AvailableModelsSnake []kiroAvailableModelItem `json:"available_models"`
	Models               []kiroAvailableModelItem `json:"models"`
	NextToken            string                   `json:"nextToken"`
	NextTokenSnake       string                   `json:"next_token"`
}

type kiroAvailableModelItem struct {
	ModelID          string `json:"modelId"`
	ModelIDSnake     string `json:"model_id"`
	ID               string `json:"id"`
	ModelName        string `json:"modelName"`
	ModelNameSnake   string `json:"model_name"`
	DisplayName      string `json:"displayName"`
	DisplayNameSnake string `json:"display_name"`
	Name             string `json:"name"`
}

func (s *AccountTestService) FetchKiroUpstreamModels(ctx context.Context, account *Account) ([]kiropkg.Model, error) {
	if account == nil {
		return nil, errors.New("account is nil")
	}
	if account.Platform != PlatformKiro {
		return nil, fmt.Errorf("not a kiro account")
	}

	token := strings.TrimSpace(account.GetCredential("access_token"))
	if account.Type == AccountTypeOAuth {
		if s == nil || s.kiroTokenProvider == nil {
			return nil, errors.New("kiro token provider not configured")
		}
		accessToken, err := s.kiroTokenProvider.GetAccessToken(ctx, account)
		if err != nil {
			return nil, fmt.Errorf("get kiro access token failed: %w", err)
		}
		token = strings.TrimSpace(accessToken)
	}
	if token == "" {
		return nil, ErrKiroModelListUnsupported
	}

	return requestKiroAvailableModels(ctx, account, kiroAPIRegion(account), strings.TrimSpace(account.GetCredential("profile_arn")), token)
}

func requestKiroAvailableModels(ctx context.Context, account *Account, region, profileArn, token string) ([]kiropkg.Model, error) {
	endpoint := resolveKiroRuntimeEndpoint(region)
	client, err := httpclient.GetClient(httpclient.Options{
		ProxyURL:           accountProxyURL(account),
		Timeout:            30 * time.Second,
		ValidateResolvedIP: true,
		AllowPrivateHosts:  isLoopbackEndpoint(endpoint),
	})
	if err != nil {
		return nil, fmt.Errorf("create kiro model list client failed: %w", err)
	}

	var all []kiroAvailableModelItem
	nextToken := ""
	for {
		resp, err := requestKiroAvailableModelsPage(ctx, client, account, endpoint, profileArn, token, nextToken)
		if err != nil {
			return nil, err
		}
		all = append(all, resp.items()...)

		nextToken = strings.TrimSpace(resp.NextToken)
		if nextToken == "" {
			nextToken = strings.TrimSpace(resp.NextTokenSnake)
		}
		if nextToken == "" {
			break
		}
	}

	return mapKiroAvailableModels(all), nil
}

func requestKiroAvailableModelsPage(ctx context.Context, client *http.Client, account *Account, endpoint, profileArn, token, nextToken string) (*kiroAvailableModelsResponse, error) {
	reqURL, err := url.Parse(endpoint + "/ListAvailableModels")
	if err != nil {
		return nil, fmt.Errorf("build kiro model list url failed: %w", err)
	}
	q := reqURL.Query()
	q.Set("origin", kiroUsageOrigin)
	q.Set("maxResults", "50")
	if profileArn != "" {
		q.Set("profileArn", profileArn)
	}
	if nextToken != "" {
		q.Set("nextToken", nextToken)
	}
	reqURL.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create kiro model list request failed: %w", err)
	}
	applyKiroModelListHeaders(req, account, token)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("kiro model list request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read kiro model list response failed: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &kiroUsageHTTPError{StatusCode: resp.StatusCode, Body: strings.TrimSpace(string(body))}
	}

	var parsed kiroAvailableModelsResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("decode kiro model list response failed: %w", err)
	}
	return &parsed, nil
}

func applyKiroModelListHeaders(req *http.Request, account *Account, token string) {
	if req == nil {
		return
	}
	accountKey := buildKiroAccountKey(account)
	machineID := buildKiroMachineID(account)
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(token))
	req.Header.Set("User-Agent", kiropkg.BuildRuntimeUserAgent(accountKey, machineID))
	req.Header.Set("X-Amz-User-Agent", kiropkg.BuildRuntimeAmzUserAgent(accountKey, machineID))
	req.Header.Set("x-amzn-codewhisperer-optout", "true")
	req.Header.Set("Amz-Sdk-Request", "attempt=1; max=3")
	req.Header.Set("Amz-Sdk-Invocation-Id", uuid.NewString())
	if account != nil {
		applyKiroConditionalHeaders(req, account)
	}
}

func (r *kiroAvailableModelsResponse) items() []kiroAvailableModelItem {
	if r == nil {
		return nil
	}
	switch {
	case len(r.AvailableModels) > 0:
		return r.AvailableModels
	case len(r.AvailableModelsSnake) > 0:
		return r.AvailableModelsSnake
	default:
		return r.Models
	}
}

func mapKiroAvailableModels(items []kiroAvailableModelItem) []kiropkg.Model {
	seen := make(map[string]struct{}, len(items))
	models := make([]kiropkg.Model, 0, len(items))
	for _, item := range items {
		id := firstNonEmptyKiroModelField(item.ModelID, item.ModelIDSnake, item.ID)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		displayName := firstNonEmptyKiroModelField(item.ModelName, item.ModelNameSnake, item.DisplayName, item.DisplayNameSnake, item.Name, id)
		models = append(models, kiropkg.Model{ID: id, Type: "model", DisplayName: displayName})
	}
	sort.Slice(models, func(i, j int) bool { return models[i].ID < models[j].ID })
	return models
}

func firstNonEmptyKiroModelField(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
