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
	// NotifyUsers lets <@user> mentions ping; @everyone and roles never do.
	NotifyUsers bool `json:"notify_users"`
	// Thread, when set, means post inside this shared thread, opening it (starter message + thread) if ID is empty.
	Thread *AnnouncementThread `json:"thread,omitempty"`
}

type AnnouncementThread struct {
	Key     string `json:"key"`
	Name    string `json:"name"`
	Starter string `json:"starter"`
	ID      string `json:"id"`
}

func (c *Client) ClaimAnnouncement(ctx context.Context, guildIDs []string) (*Announcement, error) {
	var response struct {
		Announcement *Announcement `json:"announcement"`
	}
	err := c.doJSONBody(ctx, "POST", c.endpoint("/announcements/claim"), nil, map[string]any{"guild_ids": guildIDs}, &response)
	return response.Announcement, err
}

func (c *Client) FinishAnnouncement(ctx context.Context, id, messageID, threadID string, failed bool) error {
	var response struct {
		Status string `json:"status"`
	}
	return c.doJSONBody(ctx, "POST", c.endpoint(path.Join("/announcements", id, "finish")), nil, map[string]any{"message_id": messageID, "failed": failed, "thread_id": threadID}, &response)
}

type AccessRequest struct {
	DiscordUserID   string `json:"discord_user_id"`
	DiscordUsername string `json:"discord_username"`
}

// ClaimAccessRequest returns the next access request not yet sent to the admin contact, marking it sent.
func (c *Client) ClaimAccessRequest(ctx context.Context) (*AccessRequest, error) {
	var response struct {
		AccessRequest *AccessRequest `json:"access_request"`
	}
	err := c.doJSONBody(ctx, "POST", c.endpoint("/access-requests/claim"), nil, map[string]any{}, &response)
	return response.AccessRequest, err
}
