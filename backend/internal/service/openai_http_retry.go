package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	openAIHTTPRequestRetryMaxRetries = 3
	openAIHTTPRequestRetryBaseDelay  = 150 * time.Millisecond
)

type openAIUpstreamRequestBuilder func(context.Context) (*http.Request, error)

func (s *OpenAIGatewayService) doOpenAIUpstreamWithRequestRetry(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	proxyURL string,
	passthrough bool,
	buildReq openAIUpstreamRequestBuilder,
) (*http.Response, error) {
	if buildReq == nil {
		return nil, errors.New("missing upstream request builder")
	}
	if account == nil {
		return nil, errors.New("missing account")
	}

	attempts := openAIHTTPRequestRetryMaxRetries + 1
	var lastErr error
	startedAt := time.Now()
	for attempt := 1; attempt <= attempts; attempt++ {
		upstreamCtx, releaseUpstreamCtx := detachUpstreamContext(ctx)
		req, err := buildReq(upstreamCtx)
		releaseUpstreamCtx()
		if err != nil {
			return nil, err
		}

		resp, err := s.httpUpstream.Do(req, proxyURL, account.ID, account.Concurrency)
		SetOpsLatencyMs(c, OpsUpstreamLatencyMsKey, time.Since(startedAt).Milliseconds())
		if err == nil {
			return resp, nil
		}

		lastErr = err
		s.recordOpenAIHTTPRequestErrorAttempt(c, account, passthrough, attempt, attempts, err)
		if !isRetryableOpenAIHTTPRequestError(err) || attempt >= attempts {
			break
		}
		time.Sleep(openAIHTTPRequestRetryDelay(attempt))
	}

	if isRetryableOpenAIHTTPRequestError(lastErr) {
		return nil, newOpenAIHTTPRequestFailoverError(lastErr)
	}
	return nil, fmt.Errorf("upstream request failed: %s", sanitizeUpstreamErrorMessage(lastErr.Error()))
}

func (s *OpenAIGatewayService) recordOpenAIHTTPRequestErrorAttempt(c *gin.Context, account *Account, passthrough bool, attempt, attempts int, err error) {
	if c == nil || err == nil {
		return
	}
	safeErr := sanitizeUpstreamErrorMessage(err.Error())
	setOpsUpstreamError(c, 0, safeErr, "")
	kind := "request_error"
	if attempt >= attempts {
		kind = "request_error:retry_exhausted"
	}
	appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
		Platform:           account.Platform,
		AccountID:          account.ID,
		AccountName:        account.Name,
		UpstreamStatusCode: 0,
		Passthrough:        passthrough,
		Kind:               kind,
		Message:            safeErr,
		Detail:             fmt.Sprintf("attempt=%d max_retries=%d", attempt, openAIHTTPRequestRetryMaxRetries),
	})
}

func openAIHTTPRequestRetryDelay(attempt int) time.Duration {
	if attempt <= 0 {
		return openAIHTTPRequestRetryBaseDelay
	}
	delay := openAIHTTPRequestRetryBaseDelay << (attempt - 1)
	if delay > time.Second {
		return time.Second
	}
	return delay
}

func isRetryableOpenAIHTTPRequestError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) {
		return true
	}
	if errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.ECONNREFUSED) || errors.Is(err, syscall.ETIMEDOUT) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	msg := strings.ToLower(err.Error())
	retryableMarkers := []string{
		"connection reset by peer",
		"connection refused",
		"unexpected eof",
		"server closed idle connection",
		"broken pipe",
		"connection aborted",
		"tls: use of closed connection",
		"http2: client connection lost",
	}
	for _, marker := range retryableMarkers {
		if strings.Contains(msg, marker) {
			return true
		}
	}
	return false
}

func newOpenAIHTTPRequestFailoverError(err error) *UpstreamFailoverError {
	message := "Upstream request failed"
	if err != nil {
		message = sanitizeUpstreamErrorMessage(err.Error())
	}
	body, _ := json.Marshal(gin.H{
		"error": gin.H{
			"type":    "upstream_error",
			"message": message,
		},
	})
	return &UpstreamFailoverError{
		StatusCode:   http.StatusBadGateway,
		ResponseBody: body,
	}
}
