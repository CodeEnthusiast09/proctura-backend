package submission

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/CodeEnthusiast09/proctura-backend/internal/config"
)

type Judge0Client struct {
	baseURL    string
	apiKey     string
	apiHost    string
	httpClient *http.Client
}

func NewJudge0Client(cfg config.Judge0Config) *Judge0Client {
	return &Judge0Client{
		baseURL: cfg.BaseURL,
		apiKey:  cfg.APIKey,
		apiHost: cfg.Host,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

type SubmitRequest struct {
	SourceCode     string  `json:"source_code"`
	LanguageID     int     `json:"language_id"`
	Stdin          *string `json:"stdin,omitempty"`
	ExpectedOutput *string `json:"expected_output,omitempty"`
}

type SubmitResponse struct {
	Token string `json:"token"`
}

type ResultResponse struct {
	Token         string  `json:"token"`
	Stdout        *string `json:"stdout"`
	Stderr        *string `json:"stderr"`
	CompileOutput *string `json:"compile_output"`
	Message       *string `json:"message"`
	Status        struct {
		ID          int    `json:"id"`
		Description string `json:"description"`
	} `json:"status"`
	Time   *string `json:"time"`
	Memory *int    `json:"memory"`
}

// Submit sends code to Judge0 and returns a token for polling.
func (j *Judge0Client) Submit(req SubmitRequest) (string, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequest(http.MethodPost, j.baseURL+"/submissions?base64_encoded=false&wait=false", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}

	j.setHeaders(httpReq)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := j.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("judge0 error %d: %s", resp.StatusCode, respBody)
	}

	var result SubmitResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}

	return result.Token, nil
}

// GetResult polls Judge0 for the result of a submission token.
func (j *Judge0Client) GetResult(token string) (*ResultResponse, error) {
	url := fmt.Sprintf("%s/submissions/%s?base64_encoded=false&fields=token,stdout,stderr,compile_output,message,status,time,memory", j.baseURL, token)

	httpReq, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	j.setHeaders(httpReq)

	resp, err := j.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("judge0 error %d: %s", resp.StatusCode, respBody)
	}

	var result ResultResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	return &result, nil
}

// IsProcessing returns true if the submission is still being processed.
// Judge0 status IDs: 1 = In Queue, 2 = Processing, 3+ = done
func IsProcessing(statusID int) bool {
	return statusID == 1 || statusID == 2
}

func (j *Judge0Client) setHeaders(req *http.Request) {
	req.Header.Set("x-rapidapi-key", j.apiKey)
	req.Header.Set("x-rapidapi-host", j.apiHost)
}
