package castaway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	baseURL     *url.URL
	httpClient  *http.Client
	bearerToken string
}

type Options struct {
	BearerToken string
}

type Instance struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Season    int32  `json:"season"`
	CreatedAt string `json:"created_at"`
}

type Participant struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type LeaderboardRow struct {
	ParticipantID   string `json:"participant_id"`
	ParticipantName string `json:"participant_name"`
	Score           int    `json:"score"`
	DraftPoints     int    `json:"draft_points"`
	BonusPoints     int    `json:"bonus_points"`
	TotalPoints     int    `json:"total_points"`
	PointsAvailable int    `json:"points_available"`
}

func (r LeaderboardRow) Total() int {
	if r.TotalPoints == 0 && r.Score != 0 {
		return r.Score
	}
	return r.TotalPoints
}

func (r LeaderboardRow) Draft() int {
	if r.DraftPoints == 0 && (r.TotalPoints != 0 || r.Score != 0 || r.BonusPoints != 0) {
		return r.Total() - r.BonusPoints
	}
	return r.DraftPoints
}

func (r LeaderboardRow) Bonus() int {
	return r.BonusPoints
}

type DraftPick struct {
	Position       int32  `json:"position"`
	ContestantID   string `json:"contestant_id"`
	ContestantName string `json:"contestant_name"`
}

type Draft struct {
	Participant Participant `json:"participant"`
	Picks       []DraftPick `json:"picks"`
}

type ListInstancesOptions struct {
	Season *int32
	Name   string
}

type ListParticipantsOptions struct {
	Name string
}

type APIError struct {
	StatusCode int
	Message    string
}

type apiError struct {
	Error string `json:"error"`
}

func (e *APIError) Error() string {
	if strings.TrimSpace(e.Message) == "" {
		return fmt.Sprintf("castaway api returned status %d", e.StatusCode)
	}
	return fmt.Sprintf("castaway api: %s", e.Message)
}

func NewClient(baseURL string, httpClient *http.Client, opts Options) (*Client, error) {
	parsed, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil {
		return nil, fmt.Errorf("parse base url: %w", err)
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	return &Client{baseURL: parsed, httpClient: httpClient, bearerToken: strings.TrimSpace(opts.BearerToken)}, nil
}

func (c *Client) ListInstances(ctx context.Context, opts ListInstancesOptions) ([]Instance, error) {
	requestURL := c.endpoint("/instances")
	query := requestURL.Query()
	if opts.Season != nil {
		query.Set("season", strconv.FormatInt(int64(*opts.Season), 10))
	}
	if strings.TrimSpace(opts.Name) != "" {
		query.Set("name", strings.TrimSpace(opts.Name))
	}
	requestURL.RawQuery = query.Encode()

	var response struct {
		Instances []Instance `json:"instances"`
	}
	if err := c.getJSON(ctx, requestURL, &response); err != nil {
		return nil, err
	}
	return response.Instances, nil
}

func (c *Client) GetInstance(ctx context.Context, instanceID string) (Instance, error) {
	var response struct {
		Instance Instance `json:"instance"`
	}
	if err := c.getJSON(ctx, c.endpoint(path.Join("/instances", instanceID)), &response); err != nil {
		return Instance{}, err
	}
	return response.Instance, nil
}

func (c *Client) ListParticipants(ctx context.Context, instanceID string, opts ListParticipantsOptions) ([]Participant, error) {
	requestURL := c.endpoint(path.Join("/instances", instanceID, "participants"))
	query := requestURL.Query()
	if strings.TrimSpace(opts.Name) != "" {
		query.Set("name", strings.TrimSpace(opts.Name))
	}
	requestURL.RawQuery = query.Encode()

	var response struct {
		Participants []Participant `json:"participants"`
	}
	if err := c.getJSON(ctx, requestURL, &response); err != nil {
		return nil, err
	}
	return response.Participants, nil
}

func (c *Client) GetLeaderboard(ctx context.Context, instanceID string, participantID string) ([]LeaderboardRow, error) {
	requestURL := c.endpoint(path.Join("/instances", instanceID, "leaderboard"))
	query := requestURL.Query()
	if strings.TrimSpace(participantID) != "" {
		query.Set("participant_id", strings.TrimSpace(participantID))
	}
	requestURL.RawQuery = query.Encode()

	var response struct {
		Leaderboard []LeaderboardRow `json:"leaderboard"`
	}
	if err := c.getJSON(ctx, requestURL, &response); err != nil {
		return nil, err
	}
	return response.Leaderboard, nil
}

func (c *Client) GetDraft(ctx context.Context, instanceID, participantID string) (Draft, error) {
	var draft Draft
	if err := c.getJSON(ctx, c.endpoint(path.Join("/instances", instanceID, "drafts", participantID)), &draft); err != nil {
		return Draft{}, err
	}
	return draft, nil
}

func (c *Client) endpoint(relativePath string) *url.URL {
	resolved := *c.baseURL
	resolved.Path = path.Join(c.baseURL.Path, relativePath)
	resolved.RawQuery = ""
	return &resolved
}

func (c *Client) getJSON(ctx context.Context, requestURL *url.URL, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL.String(), nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	if c.bearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.bearerToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("perform request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		var apiErr apiError
		if err := json.NewDecoder(resp.Body).Decode(&apiErr); err == nil {
			return &APIError{StatusCode: resp.StatusCode, Message: strings.TrimSpace(apiErr.Error)}
		}
		return &APIError{StatusCode: resp.StatusCode}
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}
