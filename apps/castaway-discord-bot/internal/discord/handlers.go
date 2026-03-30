package discord

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/bry-guy/srvivor/apps/castaway-discord-bot/internal/castaway"
	"github.com/bry-guy/srvivor/apps/castaway-discord-bot/internal/format"
	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
)

const commandTimeout = 5 * time.Second

type commandSpec struct {
	group   string
	name    string
	options []*discordgo.ApplicationCommandInteractionDataOption
}

func (b *Bot) handleInteraction(_ *discordgo.Session, interaction *discordgo.InteractionCreate) {
	data := interaction.ApplicationCommandData()
	if data.Name != "castaway" {
		return
	}

	switch interaction.Type {
	case discordgo.InteractionApplicationCommand:
		b.handleCommand(interaction)
	case discordgo.InteractionApplicationCommandAutocomplete:
		b.handleAutocomplete(interaction)
	}
}

func (b *Bot) handleCommand(interaction *discordgo.InteractionCreate) {
	command := parseCommandSpec(interaction.ApplicationCommandData())
	preflightCtx, preflightCancel := context.WithTimeout(context.Background(), 2*time.Second)
	ephemeral, err := b.commandShouldBeEphemeral(preflightCtx, interaction, command)
	preflightCancel()
	if err != nil {
		b.log.Warn("command preflight failed; defaulting to ephemeral response", "command", command.name, "group", command.group, "error", err)
		ephemeral = true
	}
	if err := b.deferResponse(interaction, ephemeral); err != nil {
		b.log.Error("defer interaction response", "error", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()

	content, err := b.executeCommand(ctx, interaction, command)
	if err != nil {
		b.log.Warn("command failed", "command", command.name, "group", command.group, "error", err)
		content = "Error: " + err.Error()
	}

	if err := b.editResponse(interaction, content); err != nil {
		b.log.Error("edit interaction response", "error", err)
	}
}

func (b *Bot) executeCommand(ctx context.Context, interaction *discordgo.InteractionCreate, command commandSpec) (string, error) {
	switch command.group {
	case "instance":
		switch command.name {
		case "set":
			return b.handleInstanceSet(ctx, interaction, command)
		case "show":
			return b.handleInstanceShow(ctx, interaction)
		case "unset":
			return b.handleInstanceClear(interaction, command)
		default:
			return "", fmt.Errorf("unsupported castaway instance command: %s", command.name)
		}
	default:
		switch command.name {
		case "score":
			return b.handleScore(ctx, interaction, command)
		case "scores":
			return b.handleScores(ctx, interaction, command)
		case "draft":
			return b.handleDraft(ctx, interaction, command)
		case "activities":
			return b.handleActivities(ctx, interaction, command)
		case "activity":
			return b.handleActivity(ctx, interaction, command)
		case "occurrences":
			return b.handleOccurrences(ctx, interaction, command)
		case "occurrence":
			return b.handleOccurrence(ctx, interaction, command)
		case "history":
			return b.handleHistory(ctx, interaction, command)
		case "link":
			return b.handleLink(ctx, interaction, command)
		case "unlink":
			return b.handleUnlink(ctx, interaction, command)
		case "instances":
			return b.handleInstanceList(ctx, command)
		default:
			return "", fmt.Errorf("unsupported castaway command: %s", command.name)
		}
	}
}

func (b *Bot) commandShouldBeEphemeral(ctx context.Context, interaction *discordgo.InteractionCreate, command commandSpec) (bool, error) {
	if command.group == "instance" {
		return true, nil
	}
	switch command.name {
	case "link", "unlink":
		return true, nil
	case "score", "history":
		season, err := seasonOptionValue(command)
		if err != nil {
			return true, err
		}
		instance, err := b.resolveInstance(ctx, interaction, optionString(command, "instance"), season)
		if err != nil {
			return true, err
		}
		participantOption := optionString(command, "participant")
		if participantOption == "" {
			return true, nil
		}
		participant, err := b.resolveParticipant(ctx, instance.ID, participantOption)
		if err != nil {
			return true, err
		}
		linkedParticipant, err := b.castaway.GetLinkedParticipant(ctx, instance.ID, interactionUserID(interaction))
		if err != nil {
			var apiErr *castaway.APIError
			if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
				return false, nil
			}
			return true, err
		}
		return linkedParticipant.ID == participant.ID, nil
	default:
		return false, nil
	}
}

func (b *Bot) handleScore(ctx context.Context, interaction *discordgo.InteractionCreate, command commandSpec) (string, error) {
	season, err := seasonOptionValue(command)
	if err != nil {
		return "", err
	}
	instance, err := b.resolveInstance(ctx, interaction, optionString(command, "instance"), season)
	if err != nil {
		return "", err
	}
	participant, err := b.resolveRequestedOrLinkedParticipant(ctx, interaction, instance.ID, optionString(command, "participant"))
	if err != nil {
		return "", err
	}
	rows, err := b.castaway.GetLeaderboard(ctx, instance.ID, participant.ID)
	if err != nil {
		return "", err
	}
	if len(rows) == 0 {
		return "", fmt.Errorf("no score found for %s in %s", participant.Name, format.InstanceLabel(instance))
	}
	row := rows[0]
	publicBonusPoints := row.BonusPoints
	ledger, err := b.castaway.GetBonusLedger(ctx, instance.ID, participant.ID, interactionUserID(interaction))
	if err != nil {
		return "", err
	}
	row.BonusPoints = ledger.BonusPoints
	row.TotalPoints = row.Draft() + row.BonusPoints
	row.Score = row.TotalPoints
	secretBonusPoints := row.BonusPoints - publicBonusPoints
	if secretBonusPoints < 0 {
		secretBonusPoints = 0
	}
	return format.SingleScore(instance, row, publicBonusPoints, secretBonusPoints), nil
}

func (b *Bot) handleScores(ctx context.Context, interaction *discordgo.InteractionCreate, command commandSpec) (string, error) {
	season, err := seasonOptionValue(command)
	if err != nil {
		return "", err
	}
	instance, err := b.resolveInstance(ctx, interaction, optionString(command, "instance"), season)
	if err != nil {
		return "", err
	}
	rows, err := b.castaway.GetLeaderboard(ctx, instance.ID, "")
	if err != nil {
		return "", err
	}
	if len(rows) == 0 {
		return "No leaderboard rows found yet.", nil
	}
	return format.Leaderboard(instance, rows), nil
}

func (b *Bot) handleDraft(ctx context.Context, interaction *discordgo.InteractionCreate, command commandSpec) (string, error) {
	season, err := seasonOptionValue(command)
	if err != nil {
		return "", err
	}
	instance, err := b.resolveInstance(ctx, interaction, optionString(command, "instance"), season)
	if err != nil {
		return "", err
	}
	participant, err := b.resolveRequestedOrLinkedParticipant(ctx, interaction, instance.ID, optionString(command, "participant"))
	if err != nil {
		return "", err
	}
	draft, err := b.castaway.GetDraft(ctx, instance.ID, participant.ID)
	if err != nil {
		return "", err
	}
	return format.Draft(instance, draft), nil
}

func (b *Bot) handleActivities(ctx context.Context, interaction *discordgo.InteractionCreate, command commandSpec) (string, error) {
	season, err := seasonOptionValue(command)
	if err != nil {
		return "", err
	}
	instance, err := b.resolveInstance(ctx, interaction, optionString(command, "instance"), season)
	if err != nil {
		return "", err
	}
	activities, err := b.castaway.ListActivities(ctx, instance.ID)
	if err != nil {
		return "", err
	}
	return format.ActivitiesList(instance, activities), nil
}

func (b *Bot) handleActivity(ctx context.Context, interaction *discordgo.InteractionCreate, command commandSpec) (string, error) {
	season, err := seasonOptionValue(command)
	if err != nil {
		return "", err
	}
	instance, err := b.resolveInstance(ctx, interaction, optionString(command, "instance"), season)
	if err != nil {
		return "", err
	}
	activityName := optionString(command, "activity")
	if activityName == "" {
		return "", fmt.Errorf("activity name is required")
	}
	activities, err := b.castaway.ListActivities(ctx, instance.ID)
	if err != nil {
		return "", err
	}
	activity, err := selectActivityByName(activityName, activities)
	if err != nil {
		return "", err
	}
	detail, err := b.castaway.GetActivity(ctx, activity.ID)
	if err != nil {
		return "", err
	}
	occurrences, err := b.castaway.ListOccurrences(ctx, activity.ID)
	if err != nil {
		return "", err
	}
	return format.ActivityDetail(detail, occurrences, instance), nil
}

func (b *Bot) handleOccurrences(ctx context.Context, interaction *discordgo.InteractionCreate, command commandSpec) (string, error) {
	season, err := seasonOptionValue(command)
	if err != nil {
		return "", err
	}
	instance, err := b.resolveInstance(ctx, interaction, optionString(command, "instance"), season)
	if err != nil {
		return "", err
	}
	activityName := optionString(command, "activity")
	if activityName == "" {
		return "", fmt.Errorf("activity name is required")
	}
	activities, err := b.castaway.ListActivities(ctx, instance.ID)
	if err != nil {
		return "", err
	}
	activity, err := selectActivityByName(activityName, activities)
	if err != nil {
		return "", err
	}
	occurrences, err := b.castaway.ListOccurrences(ctx, activity.ID)
	if err != nil {
		return "", err
	}
	details := make([]castaway.OccurrenceDetail, 0, len(occurrences))
	for _, occurrence := range occurrences {
		detail, detailErr := b.castaway.GetOccurrence(ctx, occurrence.ID)
		if detailErr != nil {
			detail = castaway.OccurrenceDetail{Occurrence: occurrence}
		}
		details = append(details, detail)
	}
	return format.OccurrencesList(activity, details), nil
}

func (b *Bot) handleOccurrence(ctx context.Context, interaction *discordgo.InteractionCreate, command commandSpec) (string, error) {
	season, err := seasonOptionValue(command)
	if err != nil {
		return "", err
	}
	instance, err := b.resolveInstance(ctx, interaction, optionString(command, "instance"), season)
	if err != nil {
		return "", err
	}
	activityName := optionString(command, "activity")
	if activityName == "" {
		return "", fmt.Errorf("activity name is required")
	}
	occurrenceName := optionString(command, "occurrence")
	if occurrenceName == "" {
		return "", fmt.Errorf("occurrence name is required")
	}
	activities, err := b.castaway.ListActivities(ctx, instance.ID)
	if err != nil {
		return "", err
	}
	activity, err := selectActivityByName(activityName, activities)
	if err != nil {
		return "", err
	}
	occurrences, err := b.castaway.ListOccurrences(ctx, activity.ID)
	if err != nil {
		return "", err
	}
	occurrence, err := selectOccurrenceByName(occurrenceName, occurrences)
	if err != nil {
		return "", err
	}
	detail, err := b.castaway.GetOccurrence(ctx, occurrence.ID)
	if err != nil {
		return "", err
	}
	return format.OccurrenceDetail(detail, activity), nil
}

func (b *Bot) handleHistory(ctx context.Context, interaction *discordgo.InteractionCreate, command commandSpec) (string, error) {
	season, err := seasonOptionValue(command)
	if err != nil {
		return "", err
	}
	instance, err := b.resolveInstance(ctx, interaction, optionString(command, "instance"), season)
	if err != nil {
		return "", err
	}
	participant, err := b.resolveRequestedOrLinkedParticipant(ctx, interaction, instance.ID, optionString(command, "participant"))
	if err != nil {
		return "", err
	}
	history, err := b.castaway.GetParticipantActivityHistory(ctx, instance.ID, participant.ID, interactionUserID(interaction))
	if err != nil {
		return "", err
	}
	if history.Participant.ID == "" {
		history.Participant = participant
	}
	fullInstance, fullInstanceErr := b.castaway.GetInstance(ctx, instance.ID)
	if fullInstanceErr == nil {
		history.Instance = fullInstance
	} else if history.Instance.ID == "" {
		history.Instance = instance
	}
	return format.ParticipantHistory(history), nil
}

func (b *Bot) handleLink(ctx context.Context, interaction *discordgo.InteractionCreate, command commandSpec) (string, error) {
	season, err := seasonOptionValue(command)
	if err != nil {
		return "", err
	}
	instance, err := b.resolveInstance(ctx, interaction, optionString(command, "instance"), season)
	if err != nil {
		return "", err
	}
	participant, err := b.resolveParticipant(ctx, instance.ID, optionString(command, "participant"))
	if err != nil {
		return "", err
	}
	targetDiscordUserID := optionUserID(command, "user")
	if targetDiscordUserID == "" {
		return "", fmt.Errorf("user is required")
	}
	linked, err := b.castaway.LinkDiscordUser(ctx, instance.ID, participant.ID, interactionUserID(interaction), targetDiscordUserID)
	if err != nil {
		var apiErr *castaway.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusForbidden {
			return "", fmt.Errorf("link is admin-only; ask a Castaway admin to run this command")
		}
		return "", err
	}
	return fmt.Sprintf("Linked %s to <@%s> in %s.", linked.Name, targetDiscordUserID, format.InstanceLabel(instance)), nil
}

func (b *Bot) handleUnlink(ctx context.Context, interaction *discordgo.InteractionCreate, command commandSpec) (string, error) {
	season, err := seasonOptionValue(command)
	if err != nil {
		return "", err
	}
	instance, err := b.resolveInstance(ctx, interaction, optionString(command, "instance"), season)
	if err != nil {
		return "", err
	}
	participant, err := b.resolveParticipant(ctx, instance.ID, optionString(command, "participant"))
	if err != nil {
		return "", err
	}
	unlinked, err := b.castaway.UnlinkDiscordUser(ctx, instance.ID, participant.ID, interactionUserID(interaction))
	if err != nil {
		var apiErr *castaway.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusForbidden {
			return "", fmt.Errorf("unlink is admin-only; ask a Castaway admin to run this command")
		}
		return "", err
	}
	return fmt.Sprintf("Unlinked Discord account from %s in %s.", unlinked.Name, format.InstanceLabel(instance)), nil
}

func (b *Bot) handleInstanceList(ctx context.Context, command commandSpec) (string, error) {
	season, err := seasonOptionValue(command)
	if err != nil {
		return "", err
	}
	instances, err := b.castaway.ListInstances(ctx, castaway.ListInstancesOptions{Season: season})
	if err != nil {
		return "", err
	}
	return format.InstanceList(instances), nil
}

func (b *Bot) handleInstanceSet(ctx context.Context, interaction *discordgo.InteractionCreate, command commandSpec) (string, error) {
	season, err := seasonOptionValue(command)
	if err != nil {
		return "", err
	}
	instance, err := b.resolveInstance(ctx, interaction, optionString(command, "instance"), season)
	if err != nil {
		return "", err
	}

	scope := scopeOptionValue(command)
	switch scope {
	case "guild":
		guildID := interaction.GuildID
		if guildID == "" {
			return "", fmt.Errorf("guild scope is only available inside a guild")
		}
		if !hasGuildManagePermission(interaction) {
			return "", fmt.Errorf("guild scope requires Manage Server permission")
		}
		if err := b.state.SetGuildDefault(guildID, instance.ID); err != nil {
			return "", fmt.Errorf("save guild default: %w", err)
		}
		return fmt.Sprintf("Saved guild default instance: %s", format.InstanceLabel(instance)), nil
	default:
		if err := b.state.SetUserDefault(interaction.GuildID, interactionUserID(interaction), instance.ID); err != nil {
			return "", fmt.Errorf("save user default: %w", err)
		}
		return fmt.Sprintf("Saved your default instance: %s", format.InstanceLabel(instance)), nil
	}
}

func (b *Bot) handleInstanceShow(ctx context.Context, interaction *discordgo.InteractionCreate) (string, error) {
	userID := interactionUserID(interaction)
	userDefaultID, err := b.state.GetUserDefault(interaction.GuildID, userID)
	if err != nil {
		return "", fmt.Errorf("load user default: %w", err)
	}
	guildDefaultID, err := b.state.GetGuildDefault(interaction.GuildID)
	if err != nil {
		return "", fmt.Errorf("load guild default: %w", err)
	}

	var builder strings.Builder
	builder.WriteString("**Saved instance defaults**\n")
	builder.WriteString("- You: ")
	builder.WriteString(b.describeStoredInstance(ctx, userDefaultID))
	builder.WriteString("\n- Guild: ")
	builder.WriteString(b.describeStoredInstance(ctx, guildDefaultID))
	return builder.String(), nil
}

func (b *Bot) handleInstanceClear(interaction *discordgo.InteractionCreate, command commandSpec) (string, error) {
	scope := scopeOptionValue(command)
	switch scope {
	case "guild":
		guildID := interaction.GuildID
		if guildID == "" {
			return "", fmt.Errorf("guild scope is only available inside a guild")
		}
		if !hasGuildManagePermission(interaction) {
			return "", fmt.Errorf("guild scope requires Manage Server permission")
		}
		if err := b.state.ClearGuildDefault(guildID); err != nil {
			return "", fmt.Errorf("clear guild default: %w", err)
		}
		return "Cleared guild default instance.", nil
	default:
		if err := b.state.ClearUserDefault(interaction.GuildID, interactionUserID(interaction)); err != nil {
			return "", fmt.Errorf("clear user default: %w", err)
		}
		return "Cleared your default instance.", nil
	}
}

func (b *Bot) handleAutocomplete(interaction *discordgo.InteractionCreate) {
	command := parseCommandSpec(interaction.ApplicationCommandData())
	focused := focusedOption(command.options)
	if focused == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()

	var choices []*discordgo.ApplicationCommandOptionChoice
	switch focused.Name {
	case "instance":
		choices = b.instanceChoices(ctx, command, focused.StringValue())
	case "participant":
		choices = b.participantChoices(ctx, interaction, command, focused.StringValue())
	case "activity":
		choices = b.activityChoices(ctx, interaction, command, focused.StringValue())
	case "occurrence":
		choices = b.occurrenceChoices(ctx, interaction, command, focused.StringValue())
	default:
		choices = []*discordgo.ApplicationCommandOptionChoice{}
	}

	if err := b.respondAutocomplete(interaction, choices); err != nil {
		b.log.Debug("respond autocomplete", "error", err)
	}
}

func (b *Bot) instanceChoices(ctx context.Context, command commandSpec, query string) []*discordgo.ApplicationCommandOptionChoice {
	season, err := seasonOptionValue(command)
	if err != nil {
		b.log.Debug("resolve season for instance autocomplete", "error", err)
		return emptyChoices()
	}

	instances, err := b.castaway.ListInstances(ctx, castaway.ListInstancesOptions{Season: season, Name: query})
	if err != nil {
		b.log.Debug("list instances for autocomplete", "error", err)
		return emptyChoices()
	}

	choices := make([]*discordgo.ApplicationCommandOptionChoice, 0, min(len(instances), 25))
	for _, instance := range instances {
		label := trimChoiceLabel(format.InstanceLabel(instance))
		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{Name: label, Value: instance.ID})
		if len(choices) == 25 {
			break
		}
	}
	return choices
}

func (b *Bot) participantChoices(ctx context.Context, interaction *discordgo.InteractionCreate, command commandSpec, query string) []*discordgo.ApplicationCommandOptionChoice {
	season, err := seasonOptionValue(command)
	if err != nil {
		b.log.Debug("resolve season for participant autocomplete", "error", err)
		return emptyChoices()
	}
	instance, err := b.resolveInstance(ctx, interaction, optionString(command, "instance"), season)
	if err != nil {
		b.log.Debug("resolve instance for participant autocomplete", "error", err)
		return emptyChoices()
	}
	participants, err := b.castaway.ListParticipants(ctx, instance.ID, castaway.ListParticipantsOptions{Name: query})
	if err != nil {
		b.log.Debug("list participants for autocomplete", "error", err)
		return emptyChoices()
	}

	choices := make([]*discordgo.ApplicationCommandOptionChoice, 0, min(len(participants), 25))
	for _, participant := range participants {
		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{Name: trimChoiceLabel(participant.Name), Value: participant.ID})
		if len(choices) == 25 {
			break
		}
	}
	return choices
}

func (b *Bot) activityChoices(ctx context.Context, interaction *discordgo.InteractionCreate, command commandSpec, query string) []*discordgo.ApplicationCommandOptionChoice {
	season, err := seasonOptionValue(command)
	if err != nil {
		b.log.Debug("resolve season for activity autocomplete", "error", err)
		return emptyChoices()
	}
	instance, err := b.resolveInstance(ctx, interaction, optionString(command, "instance"), season)
	if err != nil {
		b.log.Debug("resolve instance for activity autocomplete", "error", err)
		return emptyChoices()
	}
	activities, err := b.castaway.ListActivities(ctx, instance.ID)
	if err != nil {
		b.log.Debug("list activities for autocomplete", "error", err)
		return emptyChoices()
	}

	lowerQuery := strings.ToLower(strings.TrimSpace(query))
	choices := make([]*discordgo.ApplicationCommandOptionChoice, 0, min(len(activities), 25))
	for _, activity := range activities {
		if lowerQuery != "" && !strings.Contains(strings.ToLower(activity.Name), lowerQuery) {
			continue
		}
		label := trimChoiceLabel(fmt.Sprintf("%s [%s]", activity.Name, activity.ActivityType))
		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{Name: label, Value: activity.Name})
		if len(choices) == 25 {
			break
		}
	}
	return choices
}

func (b *Bot) occurrenceChoices(ctx context.Context, interaction *discordgo.InteractionCreate, command commandSpec, query string) []*discordgo.ApplicationCommandOptionChoice {
	season, err := seasonOptionValue(command)
	if err != nil {
		b.log.Debug("resolve season for occurrence autocomplete", "error", err)
		return emptyChoices()
	}
	instance, err := b.resolveInstance(ctx, interaction, optionString(command, "instance"), season)
	if err != nil {
		b.log.Debug("resolve instance for occurrence autocomplete", "error", err)
		return emptyChoices()
	}
	activities, err := b.castaway.ListActivities(ctx, instance.ID)
	if err != nil {
		b.log.Debug("list activities for occurrence autocomplete", "error", err)
		return emptyChoices()
	}
	activity, err := selectActivityByName(optionString(command, "activity"), activities)
	if err != nil {
		b.log.Debug("resolve activity for occurrence autocomplete", "error", err)
		return emptyChoices()
	}
	occurrences, err := b.castaway.ListOccurrences(ctx, activity.ID)
	if err != nil {
		b.log.Debug("list occurrences for autocomplete", "error", err)
		return emptyChoices()
	}

	lowerQuery := strings.ToLower(strings.TrimSpace(query))
	choices := make([]*discordgo.ApplicationCommandOptionChoice, 0, min(len(occurrences), 25))
	for _, occurrence := range occurrences {
		if lowerQuery != "" && !strings.Contains(strings.ToLower(occurrence.Name), lowerQuery) {
			continue
		}
		label := trimChoiceLabel(fmt.Sprintf("%s [%s]", occurrence.Name, occurrence.OccurrenceType))
		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{Name: label, Value: occurrence.Name})
		if len(choices) == 25 {
			break
		}
	}
	return choices
}

func (b *Bot) resolveInstance(ctx context.Context, interaction *discordgo.InteractionCreate, explicit string, season *int32) (castaway.Instance, error) {
	explicit = strings.TrimSpace(explicit)
	if explicit != "" {
		return b.lookupInstance(ctx, explicit, season)
	}

	userID := interactionUserID(interaction)
	if userID != "" {
		if instance, ok, err := b.instanceFromStoredDefault(ctx, interaction.GuildID, userID, season); err != nil {
			return castaway.Instance{}, err
		} else if ok {
			return instance, nil
		}
	}
	if interaction.GuildID != "" {
		if instance, ok, err := b.guildInstanceDefault(ctx, interaction.GuildID, season); err != nil {
			return castaway.Instance{}, err
		} else if ok {
			return instance, nil
		}
	}

	instances, err := b.castaway.ListInstances(ctx, castaway.ListInstancesOptions{Season: season})
	if err != nil {
		return castaway.Instance{}, err
	}
	if len(instances) == 1 {
		return instances[0], nil
	}
	if season != nil {
		if len(instances) == 0 {
			return castaway.Instance{}, fmt.Errorf("no instances found for season %d", *season)
		}
		return castaway.Instance{}, fmt.Errorf("multiple instances match season %d; specify an instance or save a default with /castaway instance set", *season)
	}
	return castaway.Instance{}, fmt.Errorf("instance is ambiguous; specify an instance or save a default with /castaway instance set")
}

func (b *Bot) lookupInstance(ctx context.Context, raw string, season *int32) (castaway.Instance, error) {
	if parsedID, err := uuid.Parse(raw); err == nil {
		instance, err := b.castaway.GetInstance(ctx, parsedID.String())
		if err != nil {
			return castaway.Instance{}, err
		}
		if season != nil && instance.Season != *season {
			return castaway.Instance{}, fmt.Errorf("instance %s is not in season %d", instance.Name, *season)
		}
		return instance, nil
	}

	instances, err := b.castaway.ListInstances(ctx, castaway.ListInstancesOptions{Season: season, Name: raw})
	if err != nil {
		return castaway.Instance{}, err
	}
	return selectInstanceByName(raw, instances)
}

func (b *Bot) resolveParticipant(ctx context.Context, instanceID, raw string) (castaway.Participant, error) {
	raw = strings.TrimSpace(raw)
	participants, err := b.castaway.ListParticipants(ctx, instanceID, castaway.ListParticipantsOptions{Name: raw})
	if err != nil {
		return castaway.Participant{}, err
	}
	if parsedID, err := uuid.Parse(raw); err == nil {
		for _, participant := range participants {
			if participant.ID == parsedID.String() {
				return participant, nil
			}
		}
		allParticipants, err := b.castaway.ListParticipants(ctx, instanceID, castaway.ListParticipantsOptions{})
		if err != nil {
			return castaway.Participant{}, err
		}
		for _, participant := range allParticipants {
			if participant.ID == parsedID.String() {
				return participant, nil
			}
		}
		return castaway.Participant{}, fmt.Errorf("participant %s was not found in this instance", parsedID.String())
	}
	return selectParticipantByName(raw, participants)
}

func (b *Bot) resolveRequestedOrLinkedParticipant(ctx context.Context, interaction *discordgo.InteractionCreate, instanceID, participantOption string) (castaway.Participant, error) {
	if strings.TrimSpace(participantOption) != "" {
		return b.resolveParticipant(ctx, instanceID, participantOption)
	}
	participant, err := b.castaway.GetLinkedParticipant(ctx, instanceID, interactionUserID(interaction))
	if err == nil {
		return participant, nil
	}
	var apiErr *castaway.APIError
	if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
		return castaway.Participant{}, fmt.Errorf("you are not linked to a participant in this instance; ask a Castaway admin to run /castaway link")
	}
	return castaway.Participant{}, err
}

func (b *Bot) instanceFromStoredDefault(ctx context.Context, guildID, userID string, season *int32) (castaway.Instance, bool, error) {
	storedID, err := b.state.GetUserDefault(guildID, userID)
	if err != nil {
		return castaway.Instance{}, false, fmt.Errorf("load user default: %w", err)
	}
	if strings.TrimSpace(storedID) == "" {
		return castaway.Instance{}, false, nil
	}
	instance, ok, err := b.fetchStoredInstance(ctx, storedID)
	if err != nil {
		return castaway.Instance{}, false, err
	}
	if !ok {
		if clearErr := b.state.ClearUserDefault(guildID, userID); clearErr != nil {
			b.log.Warn("clear stale user default", "error", clearErr, "guild_id", guildID, "user_id", userID)
		}
		return castaway.Instance{}, false, nil
	}
	if season != nil && instance.Season != *season {
		return castaway.Instance{}, false, nil
	}
	return instance, true, nil
}

func (b *Bot) guildInstanceDefault(ctx context.Context, guildID string, season *int32) (castaway.Instance, bool, error) {
	storedID, err := b.state.GetGuildDefault(guildID)
	if err != nil {
		return castaway.Instance{}, false, fmt.Errorf("load guild default: %w", err)
	}
	if strings.TrimSpace(storedID) == "" {
		return castaway.Instance{}, false, nil
	}
	instance, ok, err := b.fetchStoredInstance(ctx, storedID)
	if err != nil {
		return castaway.Instance{}, false, err
	}
	if !ok {
		if clearErr := b.state.ClearGuildDefault(guildID); clearErr != nil {
			b.log.Warn("clear stale guild default", "error", clearErr, "guild_id", guildID)
		}
		return castaway.Instance{}, false, nil
	}
	if season != nil && instance.Season != *season {
		return castaway.Instance{}, false, nil
	}
	return instance, true, nil
}

func (b *Bot) fetchStoredInstance(ctx context.Context, instanceID string) (castaway.Instance, bool, error) {
	instance, err := b.castaway.GetInstance(ctx, instanceID)
	if err == nil {
		return instance, true, nil
	}
	var apiErr *castaway.APIError
	if errors.As(err, &apiErr) && apiErr.StatusCode == 404 {
		return castaway.Instance{}, false, nil
	}
	return castaway.Instance{}, false, err
}

func (b *Bot) describeStoredInstance(ctx context.Context, instanceID string) string {
	if strings.TrimSpace(instanceID) == "" {
		return "not set"
	}
	instance, ok, err := b.fetchStoredInstance(ctx, instanceID)
	if err != nil {
		return fmt.Sprintf("lookup failed (%v)", err)
	}
	if !ok {
		return fmt.Sprintf("missing instance (%s)", instanceID)
	}
	label := format.InstanceLabel(instance)
	if instance.CurrentEpisode != nil {
		label += fmt.Sprintf(" (current: %s)", strings.TrimSpace(instance.CurrentEpisode.Label))
	}
	return label
}

func (b *Bot) deferResponse(interaction *discordgo.InteractionCreate, ephemeral bool) error {
	flags := discordgo.MessageFlags(0)
	if ephemeral {
		flags = discordgo.MessageFlagsEphemeral
	}
	return b.session.InteractionRespond(interaction.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{Flags: flags},
	})
}

func (b *Bot) editResponse(interaction *discordgo.InteractionCreate, content string) error {
	trimmed := format.TrimMessage(content)
	_, err := b.session.InteractionResponseEdit(interaction.Interaction, &discordgo.WebhookEdit{Content: &trimmed})
	return err
}

func (b *Bot) respondAutocomplete(interaction *discordgo.InteractionCreate, choices []*discordgo.ApplicationCommandOptionChoice) error {
	return b.session.InteractionRespond(interaction.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionApplicationCommandAutocompleteResult,
		Data: &discordgo.InteractionResponseData{Choices: choices},
	})
}

func parseCommandSpec(data discordgo.ApplicationCommandInteractionData) commandSpec {
	if len(data.Options) == 0 {
		return commandSpec{}
	}
	first := data.Options[0]
	if first.Type == discordgo.ApplicationCommandOptionSubCommandGroup && len(first.Options) > 0 {
		second := first.Options[0]
		return commandSpec{group: first.Name, name: second.Name, options: second.Options}
	}
	return commandSpec{name: first.Name, options: first.Options}
}

func focusedOption(options []*discordgo.ApplicationCommandInteractionDataOption) *discordgo.ApplicationCommandInteractionDataOption {
	for _, option := range options {
		if option.Focused {
			return option
		}
	}
	return nil
}

func optionString(command commandSpec, name string) string {
	for _, option := range command.options {
		if option.Name == name {
			return strings.TrimSpace(option.StringValue())
		}
	}
	return ""
}

func optionUserID(command commandSpec, name string) string {
	for _, option := range command.options {
		if option.Name != name {
			continue
		}
		if option.Type == discordgo.ApplicationCommandOptionUser {
			if userID, ok := option.Value.(string); ok {
				return strings.TrimSpace(userID)
			}
			return strings.TrimSpace(option.StringValue())
		}
		return ""
	}
	return ""
}

func seasonOptionValue(command commandSpec) (*int32, error) {
	for _, option := range command.options {
		if option.Name != "season" {
			continue
		}
		value := option.IntValue()
		if value <= 0 {
			return nil, fmt.Errorf("season must be positive")
		}
		if value > math.MaxInt32 {
			return nil, fmt.Errorf("season is too large")
		}
		season := int32(value)
		return &season, nil
	}
	return nil, nil
}

func scopeOptionValue(command commandSpec) string {
	scope := optionString(command, "scope")
	if scope == "guild" {
		return "guild"
	}
	return "me"
}

func hasGuildManagePermission(interaction *discordgo.InteractionCreate) bool {
	if interaction.Member == nil {
		return false
	}
	permissions := interaction.Member.Permissions
	return permissions&discordgo.PermissionAdministrator != 0 || permissions&discordgo.PermissionManageServer != 0
}

func interactionUserID(interaction *discordgo.InteractionCreate) string {
	if interaction.Member != nil && interaction.Member.User != nil {
		return interaction.Member.User.ID
	}
	if interaction.User != nil {
		return interaction.User.ID
	}
	return ""
}

func selectInstanceByName(query string, instances []castaway.Instance) (castaway.Instance, error) {
	if len(instances) == 0 {
		return castaway.Instance{}, fmt.Errorf("no instances matched %q", query)
	}
	if exact, ok := singleExactInstance(query, instances); ok {
		return exact, nil
	}
	if len(instances) == 1 {
		return instances[0], nil
	}
	labels := make([]string, 0, min(len(instances), 5))
	for _, instance := range instances {
		labels = append(labels, format.InstanceLabel(instance))
		if len(labels) == 5 {
			break
		}
	}
	return castaway.Instance{}, fmt.Errorf("multiple instances matched %q: %s", query, strings.Join(labels, ", "))
}

func singleExactInstance(query string, instances []castaway.Instance) (castaway.Instance, bool) {
	var match castaway.Instance
	count := 0
	for _, instance := range instances {
		if strings.EqualFold(instance.Name, query) {
			match = instance
			count++
		}
	}
	return match, count == 1
}

func selectParticipantByName(query string, participants []castaway.Participant) (castaway.Participant, error) {
	if len(participants) == 0 {
		return castaway.Participant{}, fmt.Errorf("no participants matched %q", query)
	}
	if exact, ok := singleExactParticipant(query, participants); ok {
		return exact, nil
	}
	if prefix, ok := singlePrefixParticipant(query, participants); ok {
		return prefix, nil
	}
	if len(participants) == 1 {
		return participants[0], nil
	}
	labels := make([]string, 0, min(len(participants), 5))
	for _, participant := range participants {
		labels = append(labels, participant.Name)
		if len(labels) == 5 {
			break
		}
	}
	return castaway.Participant{}, fmt.Errorf("multiple participants matched %q: %s", query, strings.Join(labels, ", "))
}

func selectActivityByName(query string, activities []castaway.Activity) (castaway.Activity, error) {
	if len(activities) == 0 {
		return castaway.Activity{}, fmt.Errorf("no activities found")
	}
	var match castaway.Activity
	count := 0
	for _, activity := range activities {
		if strings.EqualFold(activity.Name, query) {
			match = activity
			count++
		}
	}
	if count == 1 {
		return match, nil
	}

	count = 0
	lowerQuery := strings.ToLower(strings.TrimSpace(query))
	for _, activity := range activities {
		if strings.HasPrefix(strings.ToLower(activity.Name), lowerQuery) {
			match = activity
			count++
		}
	}
	if count == 1 {
		return match, nil
	}

	count = 0
	labels := make([]string, 0, min(len(activities), 5))
	for _, activity := range activities {
		if strings.Contains(strings.ToLower(activity.Name), lowerQuery) {
			match = activity
			count++
			if len(labels) < 5 {
				labels = append(labels, activity.Name)
			}
		}
	}
	if count == 1 {
		return match, nil
	}
	if count > 1 {
		return castaway.Activity{}, fmt.Errorf("multiple activities matched %q: %s", query, strings.Join(labels, ", "))
	}
	return castaway.Activity{}, fmt.Errorf("no activities matched %q", query)
}

func selectOccurrenceByName(query string, occurrences []castaway.Occurrence) (castaway.Occurrence, error) {
	if len(occurrences) == 0 {
		return castaway.Occurrence{}, fmt.Errorf("no occurrences found")
	}
	var match castaway.Occurrence
	count := 0
	for _, occurrence := range occurrences {
		if strings.EqualFold(occurrence.Name, query) {
			match = occurrence
			count++
		}
	}
	if count == 1 {
		return match, nil
	}

	count = 0
	lowerQuery := strings.ToLower(strings.TrimSpace(query))
	for _, occurrence := range occurrences {
		if strings.HasPrefix(strings.ToLower(occurrence.Name), lowerQuery) {
			match = occurrence
			count++
		}
	}
	if count == 1 {
		return match, nil
	}

	count = 0
	labels := make([]string, 0, min(len(occurrences), 5))
	for _, occurrence := range occurrences {
		if strings.Contains(strings.ToLower(occurrence.Name), lowerQuery) {
			match = occurrence
			count++
			if len(labels) < 5 {
				labels = append(labels, occurrence.Name)
			}
		}
	}
	if count == 1 {
		return match, nil
	}
	if count > 1 {
		return castaway.Occurrence{}, fmt.Errorf("multiple occurrences matched %q: %s", query, strings.Join(labels, ", "))
	}
	return castaway.Occurrence{}, fmt.Errorf("no occurrences matched %q", query)
}

func singleExactParticipant(query string, participants []castaway.Participant) (castaway.Participant, bool) {
	var match castaway.Participant
	count := 0
	for _, participant := range participants {
		if strings.EqualFold(participant.Name, query) {
			match = participant
			count++
		}
	}
	return match, count == 1
}

func singlePrefixParticipant(query string, participants []castaway.Participant) (castaway.Participant, bool) {
	lowerQuery := strings.ToLower(strings.TrimSpace(query))
	if lowerQuery == "" {
		return castaway.Participant{}, false
	}
	var match castaway.Participant
	count := 0
	for _, participant := range participants {
		if strings.HasPrefix(strings.ToLower(participant.Name), lowerQuery) {
			match = participant
			count++
		}
	}
	return match, count == 1
}

func trimChoiceLabel(label string) string {
	const limit = 100
	if len(label) <= limit {
		return label
	}
	return label[:limit-1] + "…"
}

func emptyChoices() []*discordgo.ApplicationCommandOptionChoice {
	return []*discordgo.ApplicationCommandOptionChoice{}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
