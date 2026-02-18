package jira

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

type Client struct {
	httpClient *http.Client
	baseURL    string
	authHeader string
}

func NewClient(baseURL, username, token string) *Client {
	baseURL = strings.TrimRight(baseURL, "/")
	if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
		baseURL = "https://" + baseURL
	}
	
	auth := username + ":" + token
	encodedAuth := base64.StdEncoding.EncodeToString([]byte(auth))
	
	return &Client{
		httpClient: &http.Client{},
		baseURL:    baseURL,
		authHeader: "Basic " + encodedAuth,
	}
}

func (c *Client) doRequest(method, path string, params url.Values, body interface{}) (*http.Response, error) {
	reqURL := c.baseURL + path
	if len(params) > 0 {
		reqURL += "?" + params.Encode()
	}

	var req *http.Request
	var err error

	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		req, err = http.NewRequest(method, reqURL, strings.NewReader(string(jsonBody)))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
	} else {
		req, err = http.NewRequest(method, reqURL, nil)
		if err != nil {
			return nil, err
		}
	}

	req.Header.Set("Authorization", c.authHeader)
	req.Header.Set("Accept", "application/json")

	return c.httpClient.Do(req)
}

type JQLSearchRequest struct {
	JQL           string   `json:"jql"`
	MaxResults    int      `json:"maxResults"`
	Fields        []string `json:"fields"`
	Expand        string   `json:"expand,omitempty"`
	NextPageToken string   `json:"nextPageToken,omitempty"`
}

type SearchResults struct {
	StartAt       int     `json:"startAt"`
	MaxResults    int     `json:"maxResults"`
	Total         int     `json:"total"`
	Issues        []Issue `json:"issues"`
	NextPageToken string  `json:"nextPageToken,omitempty"`
}

type Issue struct {
	Key       string                 `json:"key"`
	Fields    map[string]interface{} `json:"fields"`
	Changelog *Changelog             `json:"changelog"`
}

type Changelog struct {
	StartAt    int       `json:"startAt"`
	MaxResults int       `json:"maxResults"`
	Total      int       `json:"total"`
	Histories  []History `json:"histories"`
}

type History struct {
	ID      string   `json:"id"`
	Created string   `json:"created"`
	Items   []Item   `json:"items"`
}

type Item struct {
	Field      string `json:"field"`
	FieldType  string `json:"fieldtype"`
	From       string `json:"from"`
	FromString string `json:"fromString"`
	To         string `json:"to"`
	ToString   string `json:"toString"`
}

func (c *Client) SearchChangelogs(jql string) ([]Issue, error) {
	allIssues := []Issue{}
	maxResults := 100 // Optimized batch size for Jira Cloud (max 100)
	nextPageToken := ""

	for {
		params := url.Values{}

		reqBody := JQLSearchRequest{
			JQL:        jql,
			MaxResults: maxResults,
			Fields: []string{
				"key", "summary", "issuetype", "status", "project", "created", "resolution", "statuscategorychangedate",
				"assignee", "customfield_10028", "customfield_10603", "customfield_10943", "customfield_10641",
			},
			Expand:        "changelog",
			NextPageToken: nextPageToken,
		}
		resp, err := c.doRequest("POST", "/rest/api/3/search/jql", params, reqBody)
		if err != nil {
			return nil, err
		}
		defer func() {
			if cerr := resp.Body.Close(); cerr != nil && err == nil {
				err = cerr
			}
		}()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("jira API returned status: %s", resp.Status)
		}

		var result SearchResults
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, err
		}

		allIssues = append(allIssues, result.Issues...)

		if result.NextPageToken == "" {
			break
		}
		nextPageToken = result.NextPageToken
	}

	return allIssues, nil
}

func (c *Client) Myself() error {
	resp, err := c.doRequest("GET", "/rest/api/3/myself", nil, nil)
	if err != nil {
		return err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check failed: %s", resp.Status)
	}
	return nil
}
