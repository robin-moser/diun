package elasticsearch

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/crazy-max/diun/v4/internal/model"
	"github.com/crazy-max/diun/v4/internal/msg"
	"github.com/crazy-max/diun/v4/internal/notif/notifier"
	"github.com/crazy-max/diun/v4/pkg/utl"
)

// Client represents an active elasticsearch notification object
type Client struct {
	*notifier.Notifier
	cfg        *model.NotifElasticsearch
	meta       model.Meta
	httpClient *http.Client
}

// New creates a new elasticsearch notification instance
func New(config *model.NotifElasticsearch, meta model.Meta) notifier.Notifier {
	return notifier.Notifier{
		Handler: &Client{
			cfg:  config,
			meta: meta,
		},
	}
}

// Name returns notifier's name
func (c *Client) Name() string {
	return "elasticsearch"
}

// Send creates and sends an elasticsearch notification with an entry
func (c *Client) Send(entry model.NotifEntry) error {
	username, err := utl.GetSecret(c.cfg.Username, c.cfg.UsernameFile)
	if err != nil {
		return err
	}

	password, err := utl.GetSecret(c.cfg.Password, c.cfg.PasswordFile)
	if err != nil {
		return err
	}

	// Use the same JSON structure as webhook notifier
	message, err := msg.New(msg.Options{
		Meta:  c.meta,
		Entry: entry,
	})
	if err != nil {
		return err
	}

	body, err := message.RenderJSON()
	if err != nil {
		return err
	}

	// Parse the JSON to add the client field
	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		return err
	}

	// Add the client field from the configuration
	doc["client"] = c.cfg.Client

	// Re-marshal the JSON with the client field
	body, err = json.Marshal(doc)
	if err != nil {
		return err
	}

	// Create Elasticsearch URL
	url := fmt.Sprintf("%s://%s:%d/%s/_doc", c.cfg.Scheme, c.cfg.Host, c.cfg.Port, c.cfg.Index)

	if c.httpClient == nil {
		c.httpClient = &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: c.cfg.InsecureSkipVerify,
				},
			},
		}
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", c.meta.UserAgent)

	// Add authentication if provided
	if username != "" && password != "" {
		req.SetBasicAuth(username, password)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("elasticsearch returned status %d", resp.StatusCode)
	}

	return nil
}
