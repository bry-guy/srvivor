package castaway

import (
	"context"
	"path"
)

type Announcement struct {
	ID         string `json:"id"`
	InstanceID string `json:"instance_id"`
	GuildID    string `json:"guild_id"`
	ChannelID  string `json:"channel_id"`
	Body       string `json:"body"`
}

func (c *Client) ClaimAnnouncement(ctx context.Context, guildIDs []string) (*Announcement, error) {
	var response struct {
		Announcement *Announcement `json:"announcement"`
	}
	err := c.doJSONBody(ctx, "POST", c.endpoint("/announcements/claim"), nil, map[string]any{"guild_ids": guildIDs}, &response)
	return response.Announcement, err
}

func (c *Client) FinishAnnouncement(ctx context.Context, id, messageID string, failed bool) error {
	var response struct {
		Status string `json:"status"`
	}
	return c.doJSONBody(ctx, "POST", c.endpoint(path.Join("/announcements", id, "finish")), nil, map[string]any{"message_id": messageID, "failed": failed}, &response)
}
