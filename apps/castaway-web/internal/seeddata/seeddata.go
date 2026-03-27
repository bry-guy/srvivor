package seeddata

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type SeasonSeed struct {
	Season            int                    `json:"season"`
	InstanceName      string                 `json:"instance_name"`
	Contestants       []string               `json:"contestants"`
	Participants      []ParticipantSeed      `json:"participants"`
	Outcomes          []OutcomeSeed          `json:"outcomes"`
	ParticipantGroups []ParticipantGroupSeed `json:"participant_groups,omitempty"`
	Activities        []ActivitySeed         `json:"activities,omitempty"`
	Advantages        []AdvantageSeed        `json:"advantages,omitempty"`
}

type ParticipantGroupSeed struct {
	Name        string                `json:"name"`
	Kind        string                `json:"kind"`
	Metadata    json.RawMessage       `json:"metadata,omitempty"`
	Memberships []GroupMembershipSeed `json:"memberships,omitempty"`
}

type GroupMembershipSeed struct {
	ParticipantName string     `json:"participant_name"`
	Role            string     `json:"role,omitempty"`
	StartsAt        time.Time  `json:"starts_at"`
	EndsAt          *time.Time `json:"ends_at,omitempty"`
}

type ParticipantSeed struct {
	Name  string   `json:"name"`
	Picks []string `json:"picks"`
}

type OutcomeSeed struct {
	Position       int    `json:"position"`
	ContestantName string `json:"contestant_name,omitempty"`
}

type ActivitySeed struct {
	ActivityType           string                              `json:"activity_type"`
	Name                   string                              `json:"name"`
	Status                 string                              `json:"status,omitempty"`
	StartsAt               time.Time                           `json:"starts_at"`
	EndsAt                 *time.Time                          `json:"ends_at,omitempty"`
	Metadata               json.RawMessage                     `json:"metadata,omitempty"`
	GroupAssignments       []ActivityGroupAssignmentSeed       `json:"activity_group_assignments,omitempty"`
	ParticipantAssignments []ActivityParticipantAssignmentSeed `json:"activity_participant_assignments,omitempty"`
	Occurrences            []OccurrenceSeed                    `json:"occurrences,omitempty"`
}

type ActivityGroupAssignmentSeed struct {
	ParticipantGroupName string          `json:"participant_group_name"`
	Role                 string          `json:"role,omitempty"`
	StartsAt             time.Time       `json:"starts_at"`
	EndsAt               *time.Time      `json:"ends_at,omitempty"`
	Configuration        json.RawMessage `json:"configuration,omitempty"`
}

type ActivityParticipantAssignmentSeed struct {
	ParticipantName      string          `json:"participant_name"`
	ParticipantGroupName string          `json:"participant_group_name,omitempty"`
	Role                 string          `json:"role,omitempty"`
	StartsAt             time.Time       `json:"starts_at"`
	EndsAt               *time.Time      `json:"ends_at,omitempty"`
	Configuration        json.RawMessage `json:"configuration,omitempty"`
}

type OccurrenceSeed struct {
	OccurrenceType string                      `json:"occurrence_type"`
	Name           string                      `json:"name"`
	EffectiveAt    time.Time                   `json:"effective_at"`
	StartsAt       *time.Time                  `json:"starts_at,omitempty"`
	EndsAt         *time.Time                  `json:"ends_at,omitempty"`
	Status         string                      `json:"status,omitempty"`
	SourceRef      string                      `json:"source_ref,omitempty"`
	Metadata       json.RawMessage             `json:"metadata,omitempty"`
	Resolve        bool                        `json:"resolve,omitempty"`
	Participants   []OccurrenceParticipantSeed `json:"participants,omitempty"`
}

type OccurrenceParticipantSeed struct {
	Name                 string          `json:"name"`
	ParticipantGroupName string          `json:"participant_group_name,omitempty"`
	Role                 string          `json:"role,omitempty"`
	Result               string          `json:"result,omitempty"`
	Metadata             json.RawMessage `json:"metadata,omitempty"`
}

type AdvantageSeed struct {
	ParticipantName string          `json:"participant_name"`
	GroupName       string          `json:"group_name,omitempty"`
	AdvantageType   string          `json:"advantage_type"`
	Name            string          `json:"name"`
	Status          string          `json:"status,omitempty"`
	GrantedAt       time.Time       `json:"granted_at"`
	EffectiveAt     time.Time       `json:"effective_at"`
	EffectiveUntil  *time.Time      `json:"effective_until,omitempty"`
	Metadata        json.RawMessage `json:"metadata,omitempty"`
}

type parsedDraft struct {
	Drafter string
	Entries []draftEntry
}

type draftEntry struct {
	Position int
	Name     string
}

type rosterFile struct {
	Season      int `json:"season"`
	Contestants []struct {
		CanonicalName string `json:"canonical_name"`
	} `json:"contestants"`
}

func LoadFromLegacy(cliAppDir string) ([]SeasonSeed, error) {
	draftsDir := filepath.Join(cliAppDir, "drafts")
	seasonDirs, err := os.ReadDir(draftsDir)
	if err != nil {
		return nil, fmt.Errorf("read legacy drafts dir: %w", err)
	}

	seasons := make([]SeasonSeed, 0)
	for _, entry := range seasonDirs {
		if !entry.IsDir() {
			continue
		}

		season, err := strconv.Atoi(entry.Name())
		if err != nil || season <= 0 {
			continue
		}

		seasonDir := filepath.Join(draftsDir, entry.Name())
		seasonSeed, err := loadSeasonSeed(cliAppDir, season, seasonDir)
		if err != nil {
			return nil, err
		}
		seasons = append(seasons, seasonSeed)
	}

	sort.Slice(seasons, func(i, j int) bool {
		return seasons[i].Season < seasons[j].Season
	})
	return seasons, nil
}

func loadSeasonSeed(cliAppDir string, season int, seasonDir string) (SeasonSeed, error) {
	files, err := os.ReadDir(seasonDir)
	if err != nil {
		return SeasonSeed{}, fmt.Errorf("read season dir %s: %w", seasonDir, err)
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].Name() < files[j].Name()
	})

	participants := make([]ParticipantSeed, 0)
	outcomes := make([]OutcomeSeed, 0)
	contestantSeen := map[string]struct{}{}
	contestantOrder := make([]string, 0)

	registerContestant := func(name string) {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			return
		}
		key := strings.ToLower(trimmed)
		if _, ok := contestantSeen[key]; ok {
			return
		}
		contestantSeen[key] = struct{}{}
		contestantOrder = append(contestantOrder, trimmed)
	}

	for _, file := range files {
		if file.IsDir() || filepath.Ext(file.Name()) != ".txt" {
			continue
		}

		draftPath := filepath.Join(seasonDir, file.Name())
		draft, err := parseDraftFile(draftPath)
		if err != nil {
			return SeasonSeed{}, fmt.Errorf("parse draft file %s: %w", draftPath, err)
		}

		isFinal := strings.EqualFold(strings.TrimSuffix(file.Name(), ".txt"), "final") || strings.EqualFold(draft.Drafter, "final") || strings.EqualFold(draft.Drafter, "current")
		if isFinal {
			outcomes = make([]OutcomeSeed, 0, len(draft.Entries))
			for _, entry := range draft.Entries {
				name := strings.TrimSpace(entry.Name)
				if name != "" {
					registerContestant(name)
				}
				outcomes = append(outcomes, OutcomeSeed{Position: entry.Position, ContestantName: name})
			}
			continue
		}

		participantName := strings.TrimSpace(draft.Drafter)
		if participantName == "" {
			participantName = strings.TrimSpace(strings.TrimSuffix(file.Name(), ".txt"))
		}

		picks := make([]string, 0, len(draft.Entries))
		for _, entry := range draft.Entries {
			name := strings.TrimSpace(entry.Name)
			picks = append(picks, name)
			registerContestant(name)
		}
		participants = append(participants, ParticipantSeed{Name: participantName, Picks: picks})
	}

	rosterContestants, err := loadRosterContestants(cliAppDir, season)
	if err != nil {
		return SeasonSeed{}, err
	}
	contestants := make([]string, 0, len(rosterContestants)+len(contestantOrder))
	if len(rosterContestants) > 0 {
		for _, name := range rosterContestants {
			trimmed := strings.TrimSpace(name)
			if trimmed == "" {
				continue
			}
			key := strings.ToLower(trimmed)
			if _, ok := contestantSeen[key]; ok {
				contestants = append(contestants, trimmed)
				continue
			}
			contestants = append(contestants, trimmed)
			contestantSeen[key] = struct{}{}
		}
		for _, name := range contestantOrder {
			key := strings.ToLower(name)
			alreadyInList := false
			for _, c := range contestants {
				if strings.EqualFold(c, name) {
					alreadyInList = true
					break
				}
			}
			if !alreadyInList {
				if _, ok := contestantSeen[key]; ok {
					contestants = append(contestants, name)
				}
			}
		}
	} else {
		contestants = append(contestants, contestantOrder...)
	}

	sort.Slice(participants, func(i, j int) bool {
		return strings.ToLower(participants[i].Name) < strings.ToLower(participants[j].Name)
	})
	sort.Slice(outcomes, func(i, j int) bool {
		return outcomes[i].Position < outcomes[j].Position
	})

	return SeasonSeed{
		Season:       season,
		InstanceName: fmt.Sprintf("Historical Season %d", season),
		Contestants:  contestants,
		Participants: participants,
		Outcomes:     outcomes,
	}, nil
}

func loadRosterContestants(cliAppDir string, season int) ([]string, error) {
	rosterPath := filepath.Join(cliAppDir, "rosters", fmt.Sprintf("%d.json", season))
	_, err := os.Stat(rosterPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("stat roster file: %w", err)
	}

	payload, err := os.ReadFile(rosterPath)
	if err != nil {
		return nil, fmt.Errorf("read roster file: %w", err)
	}

	var roster rosterFile
	if err := json.Unmarshal(payload, &roster); err != nil {
		return nil, fmt.Errorf("unmarshal roster json: %w", err)
	}

	contestants := make([]string, 0, len(roster.Contestants))
	for _, contestant := range roster.Contestants {
		if strings.TrimSpace(contestant.CanonicalName) == "" {
			continue
		}
		contestants = append(contestants, strings.TrimSpace(contestant.CanonicalName))
	}
	return contestants, nil
}

func parseDraftFile(path string) (parsedDraft, error) {
	file, err := os.Open(path)
	if err != nil {
		return parsedDraft{}, err
	}
	defer file.Close()

	var result parsedDraft
	result.Entries = make([]draftEntry, 0)

	scanner := bufio.NewScanner(file)
	parsingMetadata := true
	for scanner.Scan() {
		line := scanner.Text()
		if line == "---" {
			parsingMetadata = false
			continue
		}
		if parsingMetadata {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) != 2 {
				continue
			}
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			if strings.EqualFold(key, "Drafter") {
				result.Drafter = value
			}
			continue
		}

		parts := strings.SplitN(line, ".", 2)
		if len(parts) != 2 {
			continue
		}
		position, err := strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil || position <= 0 {
			continue
		}
		name := strings.TrimSpace(parts[1])
		result.Entries = append(result.Entries, draftEntry{Position: position, Name: name})
	}
	if err := scanner.Err(); err != nil {
		return parsedDraft{}, err
	}

	sort.Slice(result.Entries, func(i, j int) bool {
		return result.Entries[i].Position < result.Entries[j].Position
	})
	return result, nil
}

func LoadFromJSON(path string) ([]SeasonSeed, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read seed file: %w", err)
	}
	seasons := make([]SeasonSeed, 0)
	if err := json.Unmarshal(payload, &seasons); err != nil {
		return nil, fmt.Errorf("unmarshal seed file: %w", err)
	}
	return seasons, nil
}

func WriteJSON(path string, seasons []SeasonSeed) error {
	payload, err := json.MarshalIndent(seasons, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal seeds: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create seed dir: %w", err)
	}
	if err := os.WriteFile(path, append(payload, '\n'), 0o600); err != nil {
		return fmt.Errorf("write seed file: %w", err)
	}
	return nil
}
