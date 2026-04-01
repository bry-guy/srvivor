package discord

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/bry-guy/srvivor/apps/castaway-discord-bot/internal/castaway"
	"github.com/bry-guy/srvivor/apps/castaway-discord-bot/internal/format"
	"github.com/bwmarrin/discordgo"
)

func (b *Bot) handlePotStatus(ctx context.Context, interaction *discordgo.InteractionCreate, command commandSpec) (string, error) {
	instance, err := b.resolveInstance(ctx, interaction, optionString(command, "instance"), nil)
	if err != nil {
		return "", err
	}
	status, err := b.castaway.GetStirThePotStatus(ctx, instance.ID, interactionUserID(interaction))
	if err != nil {
		return "", err
	}
	return format.StirThePotStatus(instance, status), nil
}

func (b *Bot) handlePotAdd(ctx context.Context, interaction *discordgo.InteractionCreate, command commandSpec) (string, error) {
	instance, err := b.resolveInstance(ctx, interaction, optionString(command, "instance"), nil)
	if err != nil {
		return "", err
	}
	points := optionInt(command, "points")
	if points <= 0 {
		return "", fmt.Errorf("points must be positive")
	}
	targetParticipantID, targetSpecified, err := b.resolveActionParticipantID(ctx, interaction, instance.ID, optionString(command, "participant"))
	if err != nil {
		return "", err
	}
	result, err := b.castaway.AddStirThePotContribution(ctx, instance.ID, interactionUserID(interaction), targetParticipantID, points)
	if err != nil {
		var apiErr *castaway.APIError
		switch {
		case errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusForbidden && targetSpecified:
			return "", fmt.Errorf("pot add with a participant name is admin-only; ask a Castaway admin to run this command")
		case errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound && !targetSpecified:
			return "", fmt.Errorf("you are not linked to a Castaway participant for this season; ask a Castaway admin to run /castaway link first")
		default:
			return "", err
		}
	}
	if err := b.publishSecretReveal(result.Participant.Name, result.RevealedSecretPoints); err != nil {
		b.log.Warn("publish secret reveal after stir the pot contribution", "participant", result.Participant.Name, "revealed_secret_points", result.RevealedSecretPoints, "error", err)
	}
	return format.StirThePotContributionResult(instance, result, !targetSpecified), nil
}

func (b *Bot) handlePotStart(ctx context.Context, interaction *discordgo.InteractionCreate, command commandSpec) (string, error) {
	instance, err := b.resolveInstance(ctx, interaction, optionString(command, "instance"), nil)
	if err != nil {
		return "", err
	}
	result, err := b.castaway.StartStirThePotRound(ctx, instance.ID, interactionUserID(interaction), "")
	if err != nil {
		var apiErr *castaway.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusForbidden {
			return "", fmt.Errorf("pot start is admin-only; ask a Castaway admin to run this command")
		}
		return "", err
	}
	return format.StirThePotStartResult(instance, result), nil
}

func (b *Bot) handleAuctionStatus(ctx context.Context, interaction *discordgo.InteractionCreate, command commandSpec) (string, error) {
	instance, err := b.resolveInstance(ctx, interaction, optionString(command, "instance"), nil)
	if err != nil {
		return "", err
	}
	status, err := b.castaway.GetAuctionStatus(ctx, instance.ID, interactionUserID(interaction))
	if err != nil {
		return "", err
	}
	return format.AuctionStatus(instance, status), nil
}

func (b *Bot) handleAuctionStart(ctx context.Context, interaction *discordgo.InteractionCreate, command commandSpec) (string, error) {
	instance, err := b.resolveInstance(ctx, interaction, optionString(command, "instance"), nil)
	if err != nil {
		return "", err
	}
	contestant, err := b.resolveContestant(ctx, instance.ID, optionString(command, "player"))
	if err != nil {
		return "", err
	}
	result, err := b.castaway.StartAuctionLot(ctx, instance.ID, interactionUserID(interaction), contestant.ID)
	if err != nil {
		var apiErr *castaway.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusForbidden {
			return "", fmt.Errorf("auction start is admin-only; ask a Castaway admin to run this command")
		}
		return "", err
	}
	return format.AuctionLotStartResult(instance, result), nil
}

func (b *Bot) handleAuctionStop(ctx context.Context, interaction *discordgo.InteractionCreate, command commandSpec) (string, error) {
	instance, err := b.resolveInstance(ctx, interaction, optionString(command, "instance"), nil)
	if err != nil {
		return "", err
	}
	contestant, err := b.resolveContestant(ctx, instance.ID, optionString(command, "player"))
	if err != nil {
		return "", err
	}
	result, err := b.castaway.StopAuctionLot(ctx, instance.ID, interactionUserID(interaction), contestant.ID)
	if err != nil {
		var apiErr *castaway.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusForbidden {
			return "", fmt.Errorf("auction stop is admin-only; ask a Castaway admin to run this command")
		}
		return "", err
	}
	return format.AuctionLotStopResult(instance, result), nil
}

func (b *Bot) handleAuctionAward(ctx context.Context, interaction *discordgo.InteractionCreate, command commandSpec) (string, error) {
	instance, err := b.resolveInstance(ctx, interaction, optionString(command, "instance"), nil)
	if err != nil {
		return "", err
	}
	contestant, err := b.resolveContestant(ctx, instance.ID, optionString(command, "player"))
	if err != nil {
		return "", err
	}
	result, err := b.castaway.RecordIndividualPonyImmunity(ctx, instance.ID, interactionUserID(interaction), contestant.ID)
	if err != nil {
		var apiErr *castaway.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusForbidden {
			return "", fmt.Errorf("recording individual immunity is admin-only; ask a Castaway admin to run this command")
		}
		return "", err
	}
	return format.IndividualPonyImmunityResult(instance, result), nil
}

func (b *Bot) handleBid(ctx context.Context, interaction *discordgo.InteractionCreate, command commandSpec) (string, error) {
	instance, err := b.resolveInstance(ctx, interaction, optionString(command, "instance"), nil)
	if err != nil {
		return "", err
	}
	contestant, err := b.resolveContestant(ctx, instance.ID, optionString(command, "player"))
	if err != nil {
		return "", err
	}
	points := optionInt(command, "points")
	if points <= 0 {
		return "", fmt.Errorf("points must be positive")
	}
	targetParticipantID, targetSpecified, err := b.resolveActionParticipantID(ctx, interaction, instance.ID, optionString(command, "participant"))
	if err != nil {
		return "", err
	}
	result, err := b.castaway.SetAuctionBid(ctx, instance.ID, contestant.ID, interactionUserID(interaction), targetParticipantID, points)
	if err != nil {
		var apiErr *castaway.APIError
		switch {
		case errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusForbidden && targetSpecified:
			return "", fmt.Errorf("bid with a participant name is admin-only; ask a Castaway admin to run this command")
		case errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound && !targetSpecified:
			return "", fmt.Errorf("you are not linked to a Castaway participant for this season; ask a Castaway admin to run /castaway link first")
		default:
			return "", err
		}
	}
	if err := b.publishSecretReveal(result.Participant.Name, result.RevealedSecretPoints); err != nil {
		b.log.Warn("publish secret reveal after auction bid", "participant", result.Participant.Name, "revealed_secret_points", result.RevealedSecretPoints, "error", err)
	}
	return format.AuctionBidResult(instance, result, !targetSpecified), nil
}

func (b *Bot) handleBids(ctx context.Context, interaction *discordgo.InteractionCreate, command commandSpec) (string, error) {
	instance, err := b.resolveInstance(ctx, interaction, optionString(command, "instance"), nil)
	if err != nil {
		return "", err
	}
	status, err := b.castaway.GetAuctionStatus(ctx, instance.ID, interactionUserID(interaction))
	if err != nil {
		return "", err
	}
	return format.BidList(instance, status), nil
}

func (b *Bot) handlePonies(ctx context.Context, interaction *discordgo.InteractionCreate, command commandSpec) (string, error) {
	instance, err := b.resolveInstance(ctx, interaction, optionString(command, "instance"), nil)
	if err != nil {
		return "", err
	}
	ponies, err := b.castaway.GetMyPonies(ctx, instance.ID, interactionUserID(interaction))
	if err != nil {
		return "", err
	}
	return format.PonyList(instance, ponies), nil
}

func (b *Bot) handleLoanStatus(ctx context.Context, interaction *discordgo.InteractionCreate, command commandSpec) (string, error) {
	instance, err := b.resolveInstance(ctx, interaction, optionString(command, "instance"), nil)
	if err != nil {
		return "", err
	}
	status, err := b.castaway.GetLoanSharkStatus(ctx, instance.ID, interactionUserID(interaction))
	if err != nil {
		return "", err
	}
	return format.LoanStatus(instance, status), nil
}

func (b *Bot) handleLoanRequest(ctx context.Context, interaction *discordgo.InteractionCreate, command commandSpec) (string, error) {
	instance, err := b.resolveInstance(ctx, interaction, optionString(command, "instance"), nil)
	if err != nil {
		return "", err
	}
	points := optionInt(command, "points")
	if points <= 0 {
		return "", fmt.Errorf("points must be positive")
	}
	status, err := b.castaway.BorrowFromLoanShark(ctx, instance.ID, interactionUserID(interaction), points)
	if err != nil {
		return "", err
	}
	return format.LoanActionResult(instance, status, "Borrowed", points), nil
}

func (b *Bot) handleLoanRepay(ctx context.Context, interaction *discordgo.InteractionCreate, command commandSpec) (string, error) {
	instance, err := b.resolveInstance(ctx, interaction, optionString(command, "instance"), nil)
	if err != nil {
		return "", err
	}
	points := optionInt(command, "points")
	if points <= 0 {
		return "", fmt.Errorf("points must be positive")
	}
	status, err := b.castaway.RepayLoanShark(ctx, instance.ID, interactionUserID(interaction), points)
	if err != nil {
		return "", err
	}
	if err := b.publishSecretReveal(status.Participant.Name, status.RevealedSecretPoints); err != nil {
		b.log.Warn("publish secret reveal after loan repayment", "participant", status.Participant.Name, "revealed_secret_points", status.RevealedSecretPoints, "error", err)
	}
	return format.LoanActionResult(instance, status, "Repaid", points), nil
}

func (b *Bot) resolveActionParticipantID(ctx context.Context, interaction *discordgo.InteractionCreate, instanceID, requestedParticipant string) (string, bool, error) {
	requestedParticipant = strings.TrimSpace(requestedParticipant)
	if requestedParticipant == "" {
		return "", false, nil
	}
	participant, err := b.resolveParticipant(ctx, instanceID, requestedParticipant)
	if err != nil {
		return "", false, err
	}
	return participant.ID, true, nil
}

func (b *Bot) publishSecretReveal(participantName string, revealedSecretPoints int) error {
	if strings.TrimSpace(b.announcementChannelID) == "" || revealedSecretPoints <= 0 || b.session == nil {
		return nil
	}
	message := format.SecretRevealAnnouncement(strings.TrimSpace(participantName), revealedSecretPoints)
	if strings.TrimSpace(message) == "" {
		return nil
	}
	_, err := b.session.ChannelMessageSend(b.announcementChannelID, message)
	return err
}
