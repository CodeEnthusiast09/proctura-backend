package cloudinary

import (
	"crypto/sha1"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/CodeEnthusiast09/proctura-backend/internal/config"
)

const recordingFolder = "proctura/recordings"

type Client struct {
	cloudName string
	apiKey    string
	apiSecret string
}

func NewClient(cfg config.CloudinaryConfig) *Client {
	return &Client{
		cloudName: cfg.CloudName,
		apiKey:    cfg.APIKey,
		apiSecret: cfg.APISecret,
	}
}

type UploadSignature struct {
	Timestamp int64  `json:"timestamp"`
	Signature string `json:"signature"`
	APIKey    string `json:"api_key"`
	CloudName string `json:"cloud_name"`
	Folder    string `json:"folder"`
}

// Sign generates a short-lived signed upload token for direct browser uploads.
func (c *Client) Sign() *UploadSignature {
	ts := time.Now().Unix()
	params := map[string]string{
		"folder":    recordingFolder,
		"timestamp": fmt.Sprintf("%d", ts),
	}
	return &UploadSignature{
		Timestamp: ts,
		Signature: c.sign(params),
		APIKey:    c.apiKey,
		CloudName: c.cloudName,
		Folder:    recordingFolder,
	}
}

func (c *Client) sign(params map[string]string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+params[k])
	}

	h := sha1.New()
	h.Write([]byte(strings.Join(parts, "&") + c.apiSecret))
	return fmt.Sprintf("%x", h.Sum(nil))
}
