# Survivor Draft

Survivor Draft is a fantasy game for predicting who are the best survivors. Would you be able to spot the strong players from the weak? Find out!

## Scoring

Scoring is based on a "Position-Adjusted" system.

Drafts are Season predictions, assigning Survivors to a Position. For example, a 3 Survivor Season Draft could be:

```
1. Curly
2. Moe
3. Larry
```

Finals are Season results, where Survivors actually finish in that Season of Survivor. For example, the above 3 Survior Season Finals could be:

```
1. Larry
2. Moe
3. Curly
```

Positions are valued at the _inverse_ of their position, from last (~18th) being worth +1 to first (1st) being worth +18.

```
1. +3
2. +2
3. +1
```

The _Survivor Position Distance_ is calculated by substracting a Survivor's _Draft Position_ from their _Final Position_ (in absolute terms). For the above Draft and Finals, we can calculate the following distances:

```
Larry:  abs(Draft (3) - Final (1)) = 2
Moe:    abs(Draft (2) - Final (2)) = 0 
Curly:  abs(Draft (1) - Final (3)) = 2
```

A Draft is scored by _subtracting_ each respective Survivor's Position Distance from their Position Value. The lowest score for a given Draft Position is 0 points. The above Draft scores as:

```
1. Curly:   +3 Value -2 Distance = +1 points
2. Moe:     +2 Value -0 Distance = +2 points
3. Larry:   +1 Value -2 Distance = +0 points

Draft Score: +3 Points
Final Score (Maximum Possible): +6 Points
```

The above scoring system _favors drafts_ which most closely _predict the top Survivors_. Since the top positions have higher value ceilings, but all positions have the same floor (0 points), more correctly predicting the top positions earns more points.


## Calculating Mid-Season Scores

Over the course of a season, Final Positions are revealed week-by-week. Each week, scores can be calculated incrementally. Consider an 8 Survivor Draft.


Let's say by Week 4, the Final Positions are:

```
1.
2.
3.
4.
5. Larry
6. Dick
7. Harry
8. Moe
```

We can score the draft for _what we know_:

```
1. Tom:     +8 Value -X Distance = +X points 
2. Dick:    +7 Value -4 Distance = +2 points 
3. Harry:   +6 Value -4 Distance = +2 points
4. Cosmo:   +5 Value -X Distance = +X points
5. Elaine:  +4 Value -X Distance = +X points
6. Larry:   +3 Value -1 Distance = +2 points
7. Moe:     +2 Value -1 Distance = +1 points
8. Curly:   +1 Value -X Distance = +X points 

Draft Score (Week 4): +7 Points
Final Score (Week 4 Max): +10 Points
```

For this Draft, we know that we have missed +3 Points so far, since a perfect Draft through Week 4 would have scored +10 Points while our Draft has only scored +7 points.

