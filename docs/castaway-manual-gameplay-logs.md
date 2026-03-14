# castaway-manual-gameplay-logs

Castaway, as a game, was largely manual until early Season 50. Seasons 44-49 were run via `srvivor`, which is now named `cli` in apps. `srvivor` primarily assisted in scoring the game.

For Season 50, "Survivor Fantasy" became "Castaway", a fully-fledged async, remote game. The features of Castaway are being developed during the Season, and not all have been implemented in the Castaway software. Castaway is played almost entirely in Discord. The features listed here are not exhaustive, and all features should be documented in `functional-requirements.md` if they are not already captured there.

Castaway can be understood through the Discord logs - game state is essentially tracked, and published through it, which represents the output of the game's features. Manual features include anything I set up as a human, typically using outside software. For instance, I used tally.so to get people's drafts this year, and I've used wordle for a few things. I'm also semi-manually scoring bonus points, since they aren't yet in the software.

I may inject into the logs myself via NOTE: implying my own commentary or explanation of what happened.

## Bonus gameplay modeling notes

### Current state

- `castaway-web` stores draft state and computes draft points.
- Bonus points are not persisted in `castaway-web` yet.
- Bonus gameplay is being coordinated and resolved manually in Discord.
- Discord posts and messages act as the operational log for what happened, who participated, and what points were awarded.

### Manual concepts in play today

#### Castaway tribes

Participants in a game instance can be grouped into Castaway tribes.

Those tribes matter for bonus gameplay because some bonus events award points to a tribe rather than to a single participant.

#### Ponies

A Castaway tribe can be assigned an in-game Survivor tribe as its pony.

Current manual interpretation:
- a Castaway tribe has a pony assignment
- if that pony tribe wins immunity
- the Castaway tribe earns a bonus point for each member of that tribe

#### Wordle challenges

There is a Wordle-style challenge where winning tribes receive bonus points.

Current manual interpretation:
- a Wordle challenge happens
- one or more Castaway tribes win
- those winning tribes receive bonus points

#### Journeys

There is now a journey mechanic modeled after Survivor.

Current manual interpretation:
- 3 players are selected by their tribes
- those selected players participate in a journey
- the current journey game is `tribal diplomacy`
- the journey has its own outcome logic and bonus-point consequences

Canonical journey prompts and rules currently live in `docs/gameplay/journey-tribal-diplomancy.md`.

### What needs to be captured from Discord for each bonus event

For every manually-scored bonus event, the eventual system needs enough information to reconstruct:

1. what happened
   - event type (`pony_immunity`, `wordle`, `journey`, etc.)
   - event name / round label
   - when it happened
2. who participated
   - participating tribes
   - participating individuals
   - role in the event (`delegate`, `winner`, `loser`, etc.)
3. what source or target was involved
   - pony assignment (`Castaway tribe -> Survivor tribe`)
   - journey game name (`tribal diplomacy`)
4. what the result was
   - winner(s)
   - loser(s)
   - any event-specific outcome details
5. what points were awarded
   - recipient participant(s)
   - recipient tribe, if points are awarded by tribe and then expanded to members
   - point value
   - reason / provenance
6. where the decision was recorded
   - Discord message URL or message ID if available
   - optional freeform notes

### Suggested manual log shape

| Field | Meaning |
| --- | --- |
| `instance` | Castaway game instance |
| `event_type` | `pony_immunity`, `wordle`, `journey`, `manual_adjustment` |
| `event_name` | Human-readable label |
| `occurred_at` | When the event happened |
| `tribes_involved` | Castaway tribes involved |
| `participants_involved` | Individual participants involved |
| `external_subjects` | Pony tribe name, journey game name, etc. |
| `result` | Winner/loser/outcome summary |
| `points_awarded` | Award rows or tribe-level awards |
| `source_ref` | Discord link/message id |
| `notes` | Freeform explanation |

### Example manual log entries

#### Example: pony immunity

- `event_type`: `pony_immunity`
- `event_name`: `Episode 2 immunity`
- `tribes_involved`: `Castaway Tribe A`
- `external_subjects`:
  - `pony_survivor_tribe = "Example Survivor Tribe"`
  - `winning_survivor_tribe = "Example Survivor Tribe"`
- `result`: `pony tribe won immunity`
- `points_awarded`: `+1 bonus point to the eligible Castaway tribe recipients`
- `source_ref`: `Discord manual scoring post`

#### Example: wordle challenge

- `event_type`: `wordle`
- `event_name`: `Week 3 wordle`
- `tribes_involved`: `Castaway Tribe A`, `Castaway Tribe B`
- `result`: `Castaway Tribe B won`
- `points_awarded`: `+1 bonus point to the winning tribe recipients`
- `source_ref`: `Discord challenge result post`

#### Example: journey

- `event_type`: `journey`
- `event_name`: `Journey 1 - Tribal Diplomacy`
- `participants_involved`:
  - `Participant 1` (`delegate`)
  - `Participant 2` (`delegate`)
  - `Participant 3` (`delegate`)
- `external_subjects`:
  - `journey_game = "tribal diplomacy"`
- `result`: `event-specific journey outcome`
- `points_awarded`: `bonus points according to journey rules`
- `source_ref`: `Discord journey resolution post`

## Season 50 Logs

### Pre-season

#### Post: Lookback

**Castaway: Season 50**

*Previously, on Castaway*

Last season we saw some twists and turns, and Keith emerged as a legend of the game, winning Season 49!

He joins Season 48 winner<@444556931827499018> , Season 47 winner <@1331006071552348341> and Season 46 winner <@1330603695594799245> as the only Castaways to ever win a season!

Will this be the season we see our first TWO TIME champ? Will players adapt to the game? Find out, on Castaway: Season 50!

🔥 🔥 🔥 

It all starts tonight at 8pm EST, mostly at Kyle and Lauren's!

#### Post: Pre-season setup

Big talk from Kenny! I guess we're getting right into it!

Welcome to Season 50 of Castaway. This season will be familiar to the game you know and love, but feature some new twists and turns. The game has changed, and it's going to be more than ever.

🔥 **Outdraft, Outluck, Outplay** 🔥 

First, take a look at your new tribes.

🍊 **Tangerine**: Cila Ponies
<@444556931827499018> 
<@1184907069531377774> 
<@699047810003107850> 
<@692053627887681570> 
Keith

🥬 **Leaf**: Kalo Ponies
<@235246238382030849> 
<@695840219819147264> 
<@693275146047324232> 
<@198239741265707008> 
<@705588098129461248> 
<@88437577341755392> 

🪷 **Lotus**: Vatu Ponies
<@191756544147324930> 
<@1331006071552348341> 
<@357928922513408004> 
<@1439733434300891237> 
<@1330603695594799245> 

This season your tribe will have a chance to earn you **_bonus points_**. When your pony tribe wins an immunity challenge, you'll earn an additional bonus point for _each member of your tribe_. 

In Castaway, tribes ride together. You might just find some other opportunities for bonus points this season - hold on to your butts 🍑 .

At 8pm EST, the drafts will open. Drafts are accepted until next Wednesday at 8pm EST. Early bird gets the worm.

As a reminder for any new players, you'll need to submit a draft of all 24 players ranked from **24 (last)** to **1 (first)**. 

The low-ranked positions are worth less than the high-ranked - it's more important to pick the winners than the losers!

For example:

```
1. Bhanu
2. Dee
...
18. David
```

Keep an eye out for a form link at 8pm EST - see you then!

#### Post: Season 50 Draft

🔥 Castaway: [Season 50 Draft is now open!](https://tally.so/r/5Bdb7Q). Woohoooo!

### Week 1

## Castaway Season 50: Week 1 Scores

The sixteen contestants have been split into three tribes - and for some of you all, your ponies paid off! ||   Jenna     || was voted off and ||         Kyle        my king|| also departed, earning _all_ players a freebie 3 points. Then, the Survivor gods smiled upon the faces of the 🪷  **Lotus** and 🥬 **Leaf** tribes, with both tribes earning immunity - and winning you all bonus points! Try harder next time, 🍊 **Tangerine**!

🍊 **Tangerine**: Cila Ponies
<@444556931827499018>: 3
<@1184907069531377774>: 3 
<@699047810003107850>: 3
<@692053627887681570>: 3
Keith: 3

🥬 **Leaf**: Kalo Ponies
<@235246238382030849>: 4 
<@695840219819147264>: 4 
<@693275146047324232>: 4
<@198239741265707008>: 4 
<@705588098129461248>: 4
<@88437577341755392>: 4

🪷 **Lotus**: Vatu Ponies
<@191756544147324930>: 4
<@1331006071552348341>: 4
<@357928922513408004>: 4 
<@1439733434300891237>: 4
<@1330603695594799245>: 4 

🔥 **That's not all!** This week, not only will you have a chance to outluck with your ponies, your tribes will compete in the first **Challenge**!

 Want to know what you're playing for? It's a bonus point. 💐 

Each member of your tribe has the opportunity to complete [**Jeff Probst's Wordle Challenge!**](
https://www.nytimes.com/games/create/wordle/ZN5WYMIKAgaZghlLMUfS77XoXJzFJAA7seVELTTIhpwAmZrlqC_XlftS6aOdjx06JAjV8Rne27S0tbm8). The top 3 scores from each tribe will be averaged, and the tribe with the best score - the fewest attempts to guess the word - will earn a bonus point!

Submit your screenshot results in this thread! **No peeking, no hinting, no cheating!** If you break the honor code, Jeff Probst will be disappointed in you and you will bring shame upon your tribe.

Everybody ready? Let's get it on!

#### Thread

Paste your result screenshots here! You can use \|\| on either side of a word to make it a ||secret||, if you fancy - but if you haven't finished, why are you looking in here yet? Go play the wordle!!

##### Responses 

Wordle puzzle created by JeffProbst 2/6

🟩🟩🟩🟨🟨⬛⬛
🟩🟩🟩🟩🟩🟩🟩

Wordle puzzle created by JeffProbst 3/6

⬛🟨⬛⬛⬛🟨⬛
🟨⬛🟨⬛🟨🟨⬛
🟩🟩🟩🟩🟩🟩🟩

Wordle puzzle created by JeffProbst 3/6

⬜🟨⬜⬜⬜⬜⬜
⬜🟨🟨⬜⬜🟨🟩
🟩🟩🟩🟩🟩🟩🟩

NOTE: ... all other players posted their wordles

### Week 2

NOTE: I manually scored the wordles, then found the winning tribes.

## Castaway Season 50: Week 2 Scores

Alright castaways, come on in! Last week we saw || Savannah || voted off the island. Take a look at your new leaderboard.

1. 🪷  <@357928922513408004> : 8 (6+2)
2. 🍊  <@1184907069531377774> : 6 (5+1)
3. 🥬  <@705588098129461248> : 6 (3+3)
3. 🥬  <@235246238382030849> : 6 (3+3)
3. 🪷  <@1330603695594799245> : 6 (3+3)
3. 🥬  <@693275146047324232> : 6 (3+3)
3. 🥬  <@198239741265707008> : 6 (3+3)
3. 🥬  <@695840219819147264> : 6 (3+3)
3. 🥬   <@88437577341755392> : 6 (3+3)
10. 🍊  <@692053627887681570> : 5 (3+2)
10. 🪷  <@191756544147324930> : 5 (3+2)
10. 🪷  <@1331006071552348341> : 5 (3+2)
10. 🪷  <@1439733434300891237> : 5 (3+2)
14. 🍊  <@699047810003107850> : 4 (3+1)
14. 🍊 Keith: 4 (3+1)
14. 🍊 <@444556931827499018>: 4 (3+1)

> Scoring format is Jeff: Total (Draft+Bonus).

Known Kyle-hater <@357928922513408004> takes the first lead of the game, followed by <@1184907069531377774> somehow in 2nd despite being dragged down by 🍊 Tangerine - followed by the rest of the his tribe rightfully at the bottom of the board.

🔥 **A boat has arrived at your beach!** 🚤  This week, each tribe **must elect by majority vote** a player to get on the boat! *Only* these daring players will find what they're playing for on the other side. **Votes open now, and close at 11pm EDT!**

> Majority vote of those who participate in each tribes vote. Pick someone who wants to play!

Everybody ready? Let's get it on!

### Post: Journey Selection

NOTE: Players voted in Discord's in-app poll.

The tangerines sent <@1184907069531377774> 🍊, the leafs sent <@198239741265707008> 🥬 , and the lotus sent <@1330603695594799245> 🪷 . Good luck players! Your journey will commence tomorrow.

### Journey in private channel #castaway-island

#### Post: Journey Intro

Welcome to <#1481728015682506944>, <@1184907069531377774>, <@198239741265707008> and <@1330603695594799245>!

### Tribal Diplomacy

_Congratulations. By making the journey, you have earned 1 bonus point for yourself. But your journey is not over._

_You must now play Tribal Diplomacy. You and the other two players will each make a choice. You may discuss your choices prior to choosing._

_You may choose to **SHARE** or **STEAL**_.

_If **all three** of you **share**, each of your tribes earns **1 bonus point**._
_If **only one** of you **steals**, that player's tribe earns **3 bonus points**. The others earn nothing._
_If **two or more** of you **steal**, the **stealers earn nothing** — but any **player who chose to share** earns **1 bonus point** for their tribe._

_Choose wisely. Trust is easy to give and hard to get back._

You **must** submit your choices by 11:59pm EDT on Friday, 3/13. I will DM each of you privately for your choice.

#### Post: Tribal Diplomacy Player Choices

All 3 decisions are in!

<@198239741265707008> chose to STEAL
<@1184907069531377774> chose to STEAL
<@1330603695594799245> chose to SHARE

The lotus tribe 🪷 will receive 1 bonus point, and the other tribes will receive 0 bonus points. For participating in the journey, each of you have earned a bonus point for yourself. 

Your journey has nearly come to an end. However, one action remains for each of you.

_Before you leave, you are being **offered a risk**._

_You **will not know** what the risk is, what you stand to gain, or what you stand to lose **until you accept it**._
_**The risk is private**. The other players will not know whether you accepted or declined._

_The choice is yours._

DM me if you choose to TAKE THE RISK or RETURN TO YOUR TRIBE by 11:59pm EDT tonight.

#### Post (in DM, privately to each player who chose risk): Lost for Wordle

### Lost for Words

_You have been awarded **3 secret bonus points**. No one else knows about them — not the other journey players, not your tribe. Players will have opportunities to use bonus points, including secret bonus points, to your advantage in the future._

_But you must earn the right to keep them._

_You must complete [Lost for Wordle](https://www.nytimes.com/games/create/wordle/0TE34KB6UX886zrber39cJdYKve8ZnhZcQzpSahLmKQT6P8EJTXqwqQRzEtSayaGM9J0HxsDPXQhoZg=). For every guess you use, you will lose 1 bonus point. All of your bonus points are at risk. You lose secret bonus points first._

_Good luck._
