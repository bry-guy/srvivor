package discord

import "github.com/bwmarrin/discordgo"

func applicationCommands() []*discordgo.ApplicationCommand {
	return []*discordgo.ApplicationCommand{
		{
			Name:        "castaway",
			Description: "Castaway fantasy draft commands",
			Options: []*discordgo.ApplicationCommandOption{
				activitiesCommand(),
				activityCommand(),
				auctionCommandGroup(),
				bidCommand(),
				bidsCommand(),
				draftCommand(),
				historyCommand(),
				instanceCommandGroup(),
				instancesCommand(),
				linkCommand(),
				loanCommandGroup(),
				occurrenceCommand(),
				occurrencesCommand(),
				poniesCommand(),
				potCommandGroup(),
				scoreCommand(),
				scoresCommand(),
				unlinkCommand(),
			},
		},
	}
}

func activitiesCommand() *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionSubCommand,
		Name:        "activities",
		Description: "List gameplay activities for an instance",
		Options: []*discordgo.ApplicationCommandOption{
			instanceOption(false),
		},
	}
}

func activityCommand() *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionSubCommand,
		Name:        "activity",
		Description: "Show one activity in detail",
		Options: []*discordgo.ApplicationCommandOption{
			activityOption(true),
			instanceOption(false),
		},
	}
}

func draftCommand() *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionSubCommand,
		Name:        "draft",
		Description: "Show a participant's draft (defaults to your linked participant)",
		Options: []*discordgo.ApplicationCommandOption{
			participantOption(false),
			instanceOption(false),
		},
	}
}

func historyCommand() *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionSubCommand,
		Name:        "history",
		Description: "Show participant activity history (defaults to your linked participant)",
		Options: []*discordgo.ApplicationCommandOption{
			participantOption(false),
			instanceOption(false),
		},
	}
}

func potCommandGroup() *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionSubCommandGroup,
		Name:        "pot",
		Description: "Stir the Pot commands",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "status",
				Description: "Show your Stir the Pot status",
				Options:     []*discordgo.ApplicationCommandOption{instanceOption(false)},
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "add",
				Description: "Add blind bonus points to Stir the Pot",
				Options:     []*discordgo.ApplicationCommandOption{pointsOption(true), participantOption(false), instanceOption(false)},
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "start",
				Description: "Admin-only: open a Stir the Pot round",
				Options:     []*discordgo.ApplicationCommandOption{instanceOption(false)},
			},
		},
	}
}

func auctionCommandGroup() *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionSubCommandGroup,
		Name:        "auction",
		Description: "Individual pony auction commands",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "status",
				Description: "Show your auction status, bids, and ponies",
				Options:     []*discordgo.ApplicationCommandOption{instanceOption(false)},
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "start",
				Description: "Admin-only: open bidding for one survivor player",
				Options:     []*discordgo.ApplicationCommandOption{contestantOption("player", "Survivor player", true), instanceOption(false)},
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "stop",
				Description: "Admin-only: close and resolve bidding for one survivor player",
				Options:     []*discordgo.ApplicationCommandOption{contestantOption("player", "Survivor player", true), instanceOption(false)},
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "award",
				Description: "Admin-only: record an individual immunity winner",
				Options:     []*discordgo.ApplicationCommandOption{contestantOption("player", "Survivor player", true), instanceOption(false)},
			},
		},
	}
}

func bidCommand() *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionSubCommand,
		Name:        "bid",
		Description: "Set your blind bid for one survivor player",
		Options: []*discordgo.ApplicationCommandOption{
			contestantOption("player", "Survivor player", true),
			pointsOption(true),
			participantOption(false),
			instanceOption(false),
		},
	}
}

func bidsCommand() *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionSubCommand,
		Name:        "bids",
		Description: "Show your open auction bids",
		Options:     []*discordgo.ApplicationCommandOption{instanceOption(false)},
	}
}

func poniesCommand() *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionSubCommand,
		Name:        "ponies",
		Description: "Show your currently owned individual ponies",
		Options:     []*discordgo.ApplicationCommandOption{instanceOption(false)},
	}
}

func loanCommandGroup() *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionSubCommandGroup,
		Name:        "loan",
		Description: "Loan Shark commands",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "status",
				Description: "Show your Loan Shark status",
				Options:     []*discordgo.ApplicationCommandOption{instanceOption(false)},
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "request",
				Description: "Borrow bonus points from the Loan Shark",
				Options:     []*discordgo.ApplicationCommandOption{pointsOption(true), instanceOption(false)},
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "repay",
				Description: "Repay bonus points to the Loan Shark",
				Options:     []*discordgo.ApplicationCommandOption{pointsOption(true), instanceOption(false)},
			},
		},
	}
}

func instanceCommandGroup() *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionSubCommandGroup,
		Name:        "instance",
		Description: "Manage instance context",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "set",
				Description: "Save a default instance for yourself or this guild",
				Options: []*discordgo.ApplicationCommandOption{
					instanceOption(true),
					scopeOption(),
				},
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "show",
				Description: "Show the current saved instance defaults",
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "unset",
				Description: "Clear a saved instance default",
				Options: []*discordgo.ApplicationCommandOption{
					scopeOption(),
				},
			},
		},
	}
}

func instancesCommand() *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionSubCommand,
		Name:        "instances",
		Description: "List available instances",
	}
}

func linkCommand() *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionSubCommand,
		Name:        "link",
		Description: "Admin-only: link a participant to a Discord user",
		Options: []*discordgo.ApplicationCommandOption{
			participantOption(true),
			userOption("user", "Discord user to link", true),
			instanceOption(false),
		},
	}
}

func occurrenceCommand() *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionSubCommand,
		Name:        "occurrence",
		Description: "Show one occurrence in detail",
		Options: []*discordgo.ApplicationCommandOption{
			activityOption(true),
			occurrenceOption(true),
			instanceOption(false),
		},
	}
}

func occurrencesCommand() *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionSubCommand,
		Name:        "occurrences",
		Description: "List occurrences for an activity",
		Options: []*discordgo.ApplicationCommandOption{
			activityOption(true),
			instanceOption(false),
		},
	}
}

func scoreCommand() *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionSubCommand,
		Name:        "score",
		Description: "Show a participant score (defaults to your linked participant)",
		Options: []*discordgo.ApplicationCommandOption{
			participantOption(false),
			instanceOption(false),
		},
	}
}

func scoresCommand() *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionSubCommand,
		Name:        "scores",
		Description: "Show the leaderboard for an instance",
		Options: []*discordgo.ApplicationCommandOption{
			instanceOption(false),
		},
	}
}

func unlinkCommand() *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionSubCommand,
		Name:        "unlink",
		Description: "Admin-only: unlink a participant's Discord user",
		Options: []*discordgo.ApplicationCommandOption{
			participantOption(true),
			instanceOption(false),
		},
	}
}

func participantOption(required bool) *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Type:         discordgo.ApplicationCommandOptionString,
		Name:         "participant",
		Description:  "Participant name",
		Required:     required,
		Autocomplete: true,
	}
}

func contestantOption(name, description string, required bool) *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Type:         discordgo.ApplicationCommandOptionString,
		Name:         name,
		Description:  description,
		Required:     required,
		Autocomplete: true,
	}
}

func pointsOption(required bool) *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionInteger,
		Name:        "points",
		Description: "Bonus points",
		Required:    required,
	}
}

func userOption(name, description string, required bool) *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionUser,
		Name:        name,
		Description: description,
		Required:    required,
	}
}

func activityOption(required bool) *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Type:         discordgo.ApplicationCommandOptionString,
		Name:         "activity",
		Description:  "Activity name",
		Required:     required,
		Autocomplete: true,
	}
}

func occurrenceOption(required bool) *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Type:         discordgo.ApplicationCommandOptionString,
		Name:         "occurrence",
		Description:  "Occurrence name",
		Required:     required,
		Autocomplete: true,
	}
}

func instanceOption(required bool) *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Type:         discordgo.ApplicationCommandOptionString,
		Name:         "instance",
		Description:  "Instance name",
		Required:     required,
		Autocomplete: true,
	}
}

func scopeOption() *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionString,
		Name:        "scope",
		Description: "Whether to save the default for you or the whole guild",
		Choices: []*discordgo.ApplicationCommandOptionChoice{
			{Name: "me", Value: "me"},
			{Name: "guild", Value: "guild"},
		},
	}
}
