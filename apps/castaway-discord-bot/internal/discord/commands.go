package discord

import "github.com/bwmarrin/discordgo"

func applicationCommands() []*discordgo.ApplicationCommand {
	return []*discordgo.ApplicationCommand{
		{
			Name:        "castaway",
			Description: "Castaway fantasy draft commands",
			Options: []*discordgo.ApplicationCommandOption{
				scoreCommand(),
				scoresCommand(),
				draftCommand(),
				activitiesCommand(),
				activityCommand(),
				occurrencesCommand(),
				occurrenceCommand(),
				historyCommand(),
				linkCommand(),
				unlinkCommand(),
				instanceCommandGroup(),
			},
		},
	}
}

func scoreCommand() *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionSubCommand,
		Name:        "score",
		Description: "Show the score for a participant",
		Options: []*discordgo.ApplicationCommandOption{
			participantOption(true),
			instanceOption(false),
			seasonOption(),
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
			seasonOption(),
		},
	}
}

func draftCommand() *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionSubCommand,
		Name:        "draft",
		Description: "Show a participant's draft",
		Options: []*discordgo.ApplicationCommandOption{
			participantOption(true),
			instanceOption(false),
			seasonOption(),
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
			seasonOption(),
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
			seasonOption(),
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
			seasonOption(),
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
			seasonOption(),
		},
	}
}

func historyCommand() *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionSubCommand,
		Name:        "history",
		Description: "Show a participant's activity history",
		Options: []*discordgo.ApplicationCommandOption{
			participantOption(true),
			instanceOption(false),
			seasonOption(),
		},
	}
}

func linkCommand() *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionSubCommand,
		Name:        "link",
		Description: "Link your Discord user to a participant",
		Options: []*discordgo.ApplicationCommandOption{
			participantOption(true),
			instanceOption(false),
			seasonOption(),
		},
	}
}

func unlinkCommand() *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionSubCommand,
		Name:        "unlink",
		Description: "Unlink your Discord user from the current instance participant",
		Options: []*discordgo.ApplicationCommandOption{
			instanceOption(false),
			seasonOption(),
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
				Name:        "list",
				Description: "List available instances",
				Options: []*discordgo.ApplicationCommandOption{
					seasonOption(),
				},
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "set",
				Description: "Save a default instance for yourself or this guild",
				Options: []*discordgo.ApplicationCommandOption{
					instanceOption(true),
					seasonOption(),
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
				Name:        "clear",
				Description: "Clear a saved instance default",
				Options: []*discordgo.ApplicationCommandOption{
					scopeOption(),
				},
			},
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

func seasonOption() *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionInteger,
		Name:        "season",
		Description: "Season number",
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
