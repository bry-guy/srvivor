package castaway

import (
	"context"
	"path"
)

type DraftThread struct {
	InstanceID string `json:"instance_id"`
	ThreadID   string `json:"thread_id"`
}

// DraftThreadResult is the API's verdict on one thread post: ignored, duplicate, saved, unchanged, or problem.
type DraftThreadResult struct {
	Status   string   `json:"status"`
	Player   string   `json:"player"`
	Problems []string `json:"problems"`
	Admins   []string `json:"admin_discord_user_ids"`
}

func (c *Client) ListDraftThreads(ctx context.Context) ([]DraftThread, error) {
	var response struct {
		Threads []DraftThread `json:"threads"`
	}
	err := c.doJSON(ctx, "GET", c.endpoint("/draft-threads"), nil, &response)
	return response.Threads, err
}

func (c *Client) PostDraftThreadMessage(ctx context.Context, threadID, messageID, version, authorID, content string) (DraftThreadResult, error) {
	var result DraftThreadResult
	body := map[string]string{"message_id": messageID, "version": version, "author_discord_user_id": authorID, "content": content}
	err := c.doJSONBody(ctx, "POST", c.endpoint(path.Join("/draft-threads", threadID, "messages")), nil, body, &result)
	return result, err
}

// FixDraftThreadMessage resubmits an admin-corrected draft post as its author's draft.
func (c *Client) FixDraftThreadMessage(ctx context.Context, threadID, messageID, version, authorID, content, adminID string) (DraftThreadResult, error) {
	var result DraftThreadResult
	body := map[string]string{"message_id": messageID, "version": version, "author_discord_user_id": authorID, "content": content, "fixed_by_discord_user_id": adminID}
	err := c.doJSONBody(ctx, "POST", c.endpoint(path.Join("/draft-threads", threadID, "messages")), nil, body, &result)
	return result, err
}
