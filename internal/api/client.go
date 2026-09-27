package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	baseURL      string
	studentToken string
	httpClient   *http.Client
}

func NewClient(baseURL, studentToken string) *Client {
	baseURL = strings.TrimRight(baseURL, "/")
	return &Client{
		baseURL:      baseURL,
		studentToken: studentToken,
		httpClient:   &http.Client{Timeout: 60 * time.Second},
	}
}

func (c *Client) newRequest(ctx context.Context, method, endpoint string, body any) (*http.Request, error) {
	url := fmt.Sprintf("%s%s", c.baseURL, endpoint)
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if c.studentToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.studentToken)
	}

	return req, nil
}

func (c *Client) do(req *http.Request, out any) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("network request failed: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("server error (%d): %s", resp.StatusCode, string(bodyBytes))
	}

	if out != nil {
		if err := json.Unmarshal(bodyBytes, out); err != nil {
			return fmt.Errorf("decoding response JSON: %w", err)
		}
	}

	return nil
}

// GetManifest fetches the public course manifest from the server.
func (c *Client) GetManifest(ctx context.Context, courseSlug string) (*CourseManifest, error) {
	endpoint := fmt.Sprintf("/v1/curriculum/%s/manifest", courseSlug)
	req, err := c.newRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	var manifest CourseManifest
	if err := c.do(req, &manifest); err != nil {
		return nil, err
	}
	return &manifest, nil
}

// GetStarter fetches the starter template files for a given level.
func (c *Client) GetStarter(ctx context.Context, courseSlug string, levelOrder int) (*StarterCodeResponse, error) {
	endpoint := fmt.Sprintf("/v1/level/%s/%d/starter", courseSlug, levelOrder)
	req, err := c.newRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	var starter StarterCodeResponse
	if err := c.do(req, &starter); err != nil {
		return nil, err
	}
	return &starter, nil
}

// GetBrief fetches the student-facing brief and public constraints for a level.
func (c *Client) GetBrief(ctx context.Context, courseSlug string, levelOrder int) (*LevelBriefResponse, error) {
	endpoint := fmt.Sprintf("/v1/level/%s/%d/brief", courseSlug, levelOrder)
	req, err := c.newRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	var brief LevelBriefResponse
	if err := c.do(req, &brief); err != nil {
		return nil, err
	}
	return &brief, nil
}

// GetHint fetches a progressive hint tier for the specified level.
func (c *Client) GetHint(ctx context.Context, courseSlug string, levelOrder, tier int) (*HintResponse, error) {
	endpoint := fmt.Sprintf("/v1/level/%s/%d/hint?tier=%d", courseSlug, levelOrder, tier)
	req, err := c.newRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	var hint HintResponse
	if err := c.do(req, &hint); err != nil {
		return nil, err
	}
	return &hint, nil
}

// Submit posts student files to the authoritative server and triggers the Dual Oracle.
func (c *Client) Submit(ctx context.Context, courseSlug string, levelOrder int, language string, files map[string]string) (*SubmitResponse, error) {
	endpoint := fmt.Sprintf("/v1/level/%s/%d/submit", courseSlug, levelOrder)
	payload := SubmitRequest{
		Language: language,
		Files:    files,
	}

	req, err := c.newRequest(ctx, http.MethodPost, endpoint, payload)
	if err != nil {
		return nil, err
	}

	var result SubmitResponse
	if err := c.do(req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
