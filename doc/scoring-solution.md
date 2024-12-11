# Scoring Points Available: A Constrained Assignment Problem

I asked ChatGPT o1 to help me understand why my scoring model for "Points Available" was incorrect. The reason, it appears, is because this scoring problem is a "Constrained Assignment" problem - i.e., I need to calculate the remaining points for each permutation of results to find the theoretical max, since assigning one Survivor to a Position affects the assignment of another Survivor.

This is what we call "Future Goals".

**Short Summary:**  
Calculating the exact "Points Available" mid-season isn’t as straightforward as subtracting known losses from a maximum theoretical score. This is because future placements are interdependent: improving one survivor’s placement might force another survivor into a worse placement. To correctly find "Points Available" at any given week, you need to consider the constraints imposed by already-known final positions and then solve an optimization problem to find the best possible arrangement of the remaining survivors. The key is to realize this is a constrained assignment problem, where each unknown survivor's final position must be chosen to maximize their potential points while respecting all known results and avoiding position conflicts.

---

**Step-by-Step Reasoning:**

1. **Identify Known Final Positions:**  
   Each week, you learn some survivors’ exact final positions. For these survivors, their score is locked in. You know precisely their Position Value, their Distance, and thus their contribution to your total score.

2. **Remove Known Survivors from the Problem:**  
   Once a survivor’s final position is known, they are no longer variables in the system. You have an exact score contribution from them. Subtract this from the maximum theoretical score (the sum of all Position Values) to understand how much total "ceiling" remains.

3. **Focus on the Unknown Survivors:**  
   The challenging part is calculating how the remaining survivors (whose final positions are still unknown) can be arranged to yield the maximum possible remaining points. Each survivor’s final score is:
   \[
   \text{Score}_i = \max(\text{Value}(D_i) - |F_i - D_i|, 0)
   \]
   where \(D_i\) is the Draft Position and \(F_i\) is the Final Position of survivor \(i\).

   Since you know the already-taken final positions (from the known survivors), the unknown survivors must fill the remaining positions. The question: **How do you assign the remaining survivors to the remaining final positions to maximize the total sum of scores?**

4. **Constraints and Interdependence:**  
   - If you place one unknown survivor near their draft position, you increase their score.  
   - But this might force another survivor into a less optimal position, increasing their distance and reducing or zeroing out their score.
   - A naive calculation that just subtracts known losses and assumes you can still achieve a theoretical best for all others independently is incorrect. You must consider that the final order is a single permutation of survivors—adjusting one survivor’s placement affects the others.

5. **Mathematical Model - An Assignment Problem:**  
   This boils down to an assignment problem (like a classic optimization problem solvable by the Hungarian algorithm or a similar method). Here’s how you frame it:
   - You have a set of unknown survivors \( S = \{s_1, s_2, ...\} \).
   - You have a set of remaining available final positions \( P = \{p_1, p_2, ...\} \).
   - For each survivor \( s_i \) and each available position \( p_j \), you can calculate the survivor’s potential score:  
     \[
     \max(\text{Value}(D_{s_i}) - |p_j - D_{s_i}|, 0).
     \]
   - Your goal: assign each survivor \( s_i \) to exactly one position \( p_j \) to maximize the sum of all survivors’ scores.

6. **Points Available Calculation:**
   - Start with the known survivors’ scores (these are fixed).
   - Solve the assignment problem for the unknown survivors to find the arrangement that yields the highest possible sum of scores for them.
   - The difference between this optimized sum and your current score is the "Points Available."
   
   In other words:
   \[
   \text{Points Available} = \text{(Optimized Maximum Possible Final Score)} - \text{(Current Score to Date)}
   \]

   Since you know some survivors have already missed their best possible outcome, the “Optimized Maximum Possible Final Score” will never be the full theoretical maximum. By solving the assignment problem for the unknown survivors, you find the best possible salvage scenario given the constraints introduced by the known results.

---

**Teaching with the Provided Example:**

In your example, you tried to simply subtract known losses from a theoretical maximum to find a mid-season "Points Available." The error came from assuming that improving Elaine’s placement does not affect Cosmo’s placement (or others). However, the order of survivors is a single permutation. If you move Elaine closer to her draft position, you might push Cosmo further away, reducing Cosmo’s score.

**Correct Approach:**

1. **Lock In Known Scores:**  
   From your Week 4 example, some survivors’ final positions are known, and their scores are fixed. Add those known scores into your current total.

2. **List Remaining Survivors and Remaining Positions:**  
   Identify which survivors do not have known final positions and which final positions are still open.

3. **Calculate a Score Matrix:**  
   For each unknown survivor and each possible final position they could still occupy, calculate their potential score if placed there.

4. **Optimize the Assignment:**  
   Use a methodical approach (trial and error for small cases, or a formal algorithm for larger ones) to find the assignment of survivors to positions that yields the highest sum of scores.

5. **Subtract Known Losses:**  
   Once you have this optimal arrangement, subtract your current score from the maximum possible score under that arrangement. The result is your true "Points Available."

**Why This Works:**  
By using a proper assignment approach, you respect the interdependencies between players’ positions. You avoid double-counting potential savings and account for the fact that helping one survivor might harm another. Thus, the "Points Available" you calculate will be accurate, reflecting all constraints.

---

**In Conclusion:**  
To accurately determine "Points Available" at any point in the season:

- Fix the known results and their scores.
- Identify which survivors are still unknown and which positions remain open.
- Treat it as an assignment problem, optimizing the distribution of these survivors to remaining positions.
- The difference between this optimized scenario and your current score is the genuine "Points Available."

This method handles the complexity and interdependencies, giving you a mathematically sound approach to calculating mid-season potential.
