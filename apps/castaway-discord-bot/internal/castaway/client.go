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

type ParticipantBonusLedger struct {
	Participant Participant        `json:"participant"`
	BonusPoints int                `json:"bonus_points"`
	Ledger      []BonusLedgerEntry `json:"ledger"`
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

type Activity struct {
	ID           string `json:"id"`
	InstanceID   string `json:"instance_id"`
	ActivityType string `json:"activity_type"`
	Name         string `json:"name"`
	Status       string `json:"status"`
	StartsAt     string `json:"starts_at"`
	EndsAt       string `json:"ends_at,omitempty"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

type ActivityGroupAssignment struct {
	ParticipantGroupID   string          `json:"participant_group_id"`
	ParticipantGroupName string          `json:"participant_group_name"`
	Role                 string          `json:"role"`
	StartsAt             string          `json:"starts_at"`
	EndsAt               string          `json:"ends_at,omitempty"`
	Configuration        json.RawMessage `json:"configuration,omitempty"`
}

type ActivityParticipantAssignment struct {
	ParticipantID        string          `json:"participant_id"`
	ParticipantName      string          `json:"participant_name"`
	ParticipantGroupID   string          `json:"participant_group_id,omitempty"`
	ParticipantGroupName string          `json:"participant_group_name,omitempty"`
	Role                 string          `json:"role"`
	StartsAt             string          `json:"starts_at"`
	EndsAt               string          `json:"ends_at,omitempty"`
	Configuration        json.RawMessage `json:"configuration,omitempty"`
}

type ActivityDetail struct {
	Activity               Activity                        `json:"activity"`
	GroupAssignments       []ActivityGroupAssignment       `json:"group_assignments"`
	ParticipantAssignments []ActivityParticipantAssignment `json:"participant_assignments"`
}

type Occurrence struct {
	ID             string `json:"id"`
	ActivityID     string `json:"activity_id"`
	OccurrenceType string `json:"occurrence_type"`
	Name           string `json:"name"`
	EffectiveAt    string `json:"effective_at"`
	StartsAt       string `json:"starts_at,omitempty"`
	EndsAt         string `json:"ends_at,omitempty"`
	Status         string `json:"status"`
	SourceRef      string `json:"source_ref,omitempty"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}

type OccurrenceParticipant struct {
	ParticipantID        string          `json:"participant_id"`
	ParticipantName      string          `json:"participant_name"`
	ParticipantGroupID   string          `json:"participant_group_id,omitempty"`
	ParticipantGroupName string          `json:"participant_group_name,omitempty"`
	Role                 string          `json:"role"`
	Result               string          `json:"result,omitempty"`
	Metadata             json.RawMessage `json:"metadata,omitempty"`
}

type OccurrenceGroup struct {
	ParticipantGroupID   string          `json:"participant_group_id"`
	ParticipantGroupName string          `json:"participant_group_name"`
	Role                 string          `json:"role"`
	Result               string          `json:"result,omitempty"`
	Metadata             json.RawMessage `json:"metadata,omitempty"`
}

type BonusLedgerEntry struct {
	ID              string          `json:"id"`
	ActivityID      string          `json:"activity_id,omitempty"`
	ActivityName    string          `json:"activity_name,omitempty"`
	ActivityType    string          `json:"activity_type,omitempty"`
	OccurrenceID    string          `json:"activity_occurrence_id,omitempty"`
	OccurrenceName  string          `json:"occurrence_name,omitempty"`
	OccurrenceType  string          `json:"occurrence_type,omitempty"`
	ParticipantID   string          `json:"participant_id,omitempty"`
	ParticipantName string          `json:"participant_name,omitempty"`
	SourceGroupID   string          `json:"source_group_id,omitempty"`
	SourceGroupName string          `json:"source_group_name,omitempty"`
	EntryKind       string          `json:"entry_kind"`
	Points          int             `json:"points"`
	Visibility      string          `json:"visibility"`
	Reason          string          `json:"reason"`
	EffectiveAt     string          `json:"effective_at"`
	AwardKey        string          `json:"award_key,omitempty"`
	CreatedAt       string          `json:"created_at,omitempty"`
	Metadata        json.RawMessage `json:"metadata,omitempty"`
}

type OccurrenceDetail struct {
	Occurrence   Occurrence              `json:"occurrence"`
	Participants []OccurrenceParticipant `json:"participants"`
	Groups       []OccurrenceGroup       `json:"groups"`
	Ledger       []BonusLedgerEntry      `json:"ledger"`
}

type ParticipantOccurrenceInvolvement struct {
	ID                   int64           `json:"id,omitempty"`
	OccurrenceID         string          `json:"activity_occurrence_id,omitempty"`
	ParticipantID        string          `json:"participant_id,omitempty"`
	ParticipantGroupID   string          `json:"participant_group_id,omitempty"`
	ParticipantGroupName string          `json:"participant_group_name,omitempty"`
	Role                 string          `json:"role,omitempty"`
	Result               string          `json:"result,omitempty"`
	Metadata             json.RawMessage `json:"metadata,omitempty"`
	CreatedAt            string          `json:"created_at,omitempty"`
}

type ParticipantActivityHistoryOccurrence struct {
	Occurrence  Occurrence                        `json:"occurrence"`
	Involvement *ParticipantOccurrenceInvolvement `json:"involvement,omitempty"`
	Ledger      []BonusLedgerEntry                `json:"ledger"`
}

type ParticipantActivityHistoryActivity struct {
	Activity    Activity                               `json:"activity"`
	Occurrences []ParticipantActivityHistoryOccurrence `json:"occurrences"`
}

type ParticipantActivityHistory struct {
	Participant Participant                          `json:"participant"`
	Instance    Instance                             `json:"instance"`
	Activities  []ParticipantActivityHistoryActivity `json:"activities"`
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
	if err := c.getJSON(ctx, requestURL, nil, &response); err != nil {
		return nil, err
	}
	return response.Instances, nil
}

func (c *Client) GetInstance(ctx context.Context, instanceID string) (Instance, error) {
	var response struct {
		Instance Instance `json:"instance"`
	}
	if err := c.getJSON(ctx, c.endpoint(path.Join("/instances", instanceID)), nil, &response); err != nil {
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
	if err := c.getJSON(ctx, requestURL, nil, &response); err != nil {
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
	if err := c.getJSON(ctx, requestURL, nil, &response); err != nil {
		return nil, err
	}
	return response.Leaderboard, nil
}

func (c *Client) ListActivities(ctx context.Context, instanceID string) ([]Activity, error) {
	var response struct {
		Activities []Activity `json:"activities"`
	}
	if err := c.getJSON(ctx, c.endpoint(path.Join("/instances", instanceID, "activities")), nil, &response); err != nil {
		return nil, err
	}
	return response.Activities, nil
}

func (c *Client) GetActivity(ctx context.Context, activityID string) (ActivityDetail, error) {
	var detail ActivityDetail
	if err := c.getJSON(ctx, c.endpoint(path.Join("/activities", activityID)), nil, &detail); err != nil {
		return ActivityDetail{}, err
	}
	return detail, nil
}

func (c *Client) ListOccurrences(ctx context.Context, activityID string) ([]Occurrence, error) {
	var response struct {
		Occurrences []Occurrence `json:"occurrences"`
	}
	if err := c.getJSON(ctx, c.endpoint(path.Join("/activities", activityID, "occurrences")), nil, &response); err != nil {
		return nil, err
	}
	return response.Occurrences, nil
}

func (c *Client) GetOccurrence(ctx context.Context, occurrenceID string) (OccurrenceDetail, error) {
	var detail OccurrenceDetail
	if err := c.getJSON(ctx, c.endpoint(path.Join("/occurrences", occurrenceID)), nil, &detail); err != nil {
		return OccurrenceDetail{}, err
	}
	return detail, nil
}

func (c *Client) GetParticipantActivityHistory(ctx context.Context, instanceID, participantID, discordUserID string) (ParticipantActivityHistory, error) {
	var detail ParticipantActivityHistory
	headers := requestHeadersForDiscordUser(discordUserID)
	if err := c.getJSON(ctx, c.endpoint(path.Join("/instances", instanceID, "participants", participantID, "activity-history")), headers, &detail); err != nil {
		return ParticipantActivityHistory{}, err
	}
	return detail, nil
}

func (c *Client) GetBonusLedger(ctx context.Context, instanceID, participantID, discordUserID string) (ParticipantBonusLedger, error) {
	var detail ParticipantBonusLedger
	headers := requestHeadersForDiscordUser(discordUserID)
	if err := c.getJSON(ctx, c.endpoint(path.Join("/instances", instanceID, "participants", participantID, "bonus-ledger")), headers, &detail); err != nil {
		return ParticipantBonusLedger{}, err
	}
	return detail, nil
}

func (c *Client) GetLinkedParticipant(ctx context.Context, instanceID, discordUserID string) (Participant, error) {
	var response struct {
		Participant Participant `json:"participant"`
	}
	headers := requestHeadersForDiscordUser(discordUserID)
	if err := c.getJSON(ctx, c.endpoint(path.Join("/instances", instanceID, "participants", "me")), headers, &response); err != nil {
		return Participant{}, err
	}
	return response.Participant, nil
}

func (c *Client) LinkDiscordUser(ctx context.Context, instanceID, participantID, discordUserID string) (Participant, error) {
	var response struct {
		Participant Participant `json:"participant"`
	}
	headers := requestHeadersForDiscordUser(discordUserID)
	if err := c.doJSON(ctx, http.MethodPut, c.endpoint(path.Join("/instances", instanceID, "participants", participantID, "discord-link")), headers, &response); err != nil {
		return Participant{}, err
	}
	return response.Participant, nil
}

func (c *Client) UnlinkDiscordUser(ctx context.Context, instanceID, participantID, discordUserID string) (Participant, error) {
	var response struct {
		Participant Participant `json:"participant"`
	}
	headers := requestHeadersForDiscordUser(discordUserID)
	if err := c.doJSON(ctx, http.MethodDelete, c.endpoint(path.Join("/instances", instanceID, "participants", participantID, "discord-link")), headers, &response); err != nil {
		return Participant{}, err
	}
	return response.Participant, nil
}

func (c *Client) GetDraft(ctx context.Context, instanceID, participantID string) (Draft, error) {
	var draft Draft
	if err := c.getJSON(ctx, c.endpoint(path.Join("/instances", instanceID, "drafts", participantID)), nil, &draft); err != nil {
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

func (c *Client) getJSON(ctx context.Context, requestURL *url.URL, headers map[string]string, out any) error {
	return c.doJSON(ctx, http.MethodGet, requestURL, headers, out)
}

func (c *Client) doJSON(ctx context.Context, method string, requestURL *url.URL, headers map[string]string, out any) error {
	req, err := http.NewRequestWithContext(ctx, method, requestURL.String(), nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	if c.bearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.bearerToken)
	}
	for key, value := range headers {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		req.Header.Set(key, trimmed)
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

func requestHeadersForDiscordUser(discordUserID string) map[string]string {
	if strings.TrimSpace(discordUserID) == "" {
		return nil
	}
	return map[string]string{"X-Discord-User-ID": strings.TrimSpace(discordUserID)}
}
