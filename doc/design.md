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

## Calculating Mid-Season Points Available

Applying the same principle, we can calculate the overall Points Available for a given Draft, because we know that a perfect Draft will be worth +36 Points.

```
Draft Score         (Week 4):       +7 Points
Final Score         (Week 4 Max):   +10 Points
Draft Points Missed (Week 4):       +3 Points
Perfect Score       (Max):          +36 Points

Points Available: 
+36 Points  (Max) 
-10 Points  (Week 4 Max) 
-3 Points   (Draft Points Missed) 
= 
23 Points
```

This Draft can still an additional 23 Points, _in theory_. To do so, the final positions would all need to be correct. Let's give this Draft the best finish we can:

```
1. Tom
2. Curly
3. Cosmo
4. Elaine
5. Larry
6. Dick
7. Harry
8. Moe
```

However, this Draft is already doomed to be _not perfect_, because Positions have already been incorrectly predicted, and therefor more potential points are off the board. So, it's actual Maximum Points left by Week 4 is 20 Points:

```
1. Tom:     +8 Value -0 Distance = +8 points 
2. Dick:    +7 Value -4 Distance = +2 points 
3. Harry:   +6 Value -4 Distance = +2 points
4. Cosmo:   +5 Value -1 Distance = +4 points
5. Elaine:  +4 Value -1 Distance = +3 points
6. Larry:   +3 Value -1 Distance = +2 points
7. Moe:     +2 Value -1 Distance = +1 points
8. Curly:   +1 Value -6 Distance = +0 points 

Draft Score (Season): 22 Points
```

This implies a Week 4 Points Remaining of +15 Points. Why? Because we also know that certain Survivors have already missed their position at Week 4!

Again, the Week 4 Finals:
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

Re-scoring our Week 4 Draft, adjusting for Survivor missed positions:

```
1. Tom:     +8 Value -(min 0) Distance = +X points 
2. Dick:    +7 Value -4 Distance = +2 points 
3. Harry:   +6 Value -4 Distance = +2 points
4. Cosmo:   +5 Value -(min 0) Distance = +X points
5. Elaine:  +4 Value -(min 1) Distance = +3-0(4-1) points
6. Larry:   +3 Value -1 Distance = +2 points
7. Moe:     +2 Value -1 Distance = +1 points
8. Curly:   +1 Value -(min 4) Distance = +0(1-4=0) points 

Draft Score (Week 4): +7 Points
Final Score (Week 4 Max): +10 Points
```

In this situation, we already know that Curly will score 0 Points, because his Draft Position will be greater distance away than his Position Value. Additionally, we know Elaine has already lost at _least_ 1 point, since her Draft Position will be at least 1 distance away from her Final Position. She will score between 3 and 0 points.

So, our best attempt to calculate Points Available:


```
Draft Score         (Week 4):       +7 Points
Final Score         (Week 4 Max):   +10 Points
Draft Points Missed (Week 4):       +3 Points
Perfect Score       (Max):          +36 Points
Known Losses        (Week 4):       +1 (Curly) +1 (Elaine)

Points Available: 
+36 Points  (Max) 
-10 Points  (Week 4 Max) 
-3 Points   (Draft Points Missed) 
-2 Points   (Known Losses)
= 
21 Points
```

This calculation is _incorrect_. This is because if Elaine were to take her best possible spot (a distance of 1 away from final), Cosmo would now be at least a distance of 2 away. Unfortunately, this is a _hard problem_ for me to solve (see [Scoring Points Available Solution](./scoring-points-available.md), so instead we will refer to this as _Potential Points Available_.

Potential Points Available is a proximate indicator of _opportunity_ remaining for a Draft.

