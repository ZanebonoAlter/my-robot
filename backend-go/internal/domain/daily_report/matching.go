package daily_report

// Assignment represents a matched pair in the bipartite assignment.
type Assignment struct {
	Row int
	Col int
}

// hungarianAssignment finds the minimum-cost assignment in a square cost matrix
// using the O(n³) Hungarian (Kuhn-Munkres) algorithm with potentials.
//
// Input must be a square matrix; the caller is responsible for padding
// non-square inputs (e.g., adding rows/cols of large sentinel values).
//
// Returns nil for empty or nil input.
func hungarianAssignment(cost [][]float64) []Assignment {
	n := len(cost)
	if n == 0 {
		return nil
	}
	for _, row := range cost {
		if len(row) != n {
			return nil // non-square — caller should pad
		}
	}

	// 1-indexed arrays (index 0 unused)
	u := make([]float64, n+1) // potential for rows
	v := make([]float64, n+1) // potential for cols
	p := make([]int, n+1)     // p[j] = row assigned to col j
	way := make([]int, n+1)   // way[j] = preceding column in augmenting path

	for i := 1; i <= n; i++ {
		p[0] = i
		j0 := 0
		minv := make([]float64, n+1)
		used := make([]bool, n+1)
		for j := range minv {
			minv[j] = 1e18
		}

		for p[j0] != 0 {
			used[j0] = true
			i0 := p[j0]
			delta := 1e18
			j1 := 0

			for j := 1; j <= n; j++ {
				if used[j] {
					continue
				}
				cur := cost[i0-1][j-1] - u[i0] - v[j]
				if cur < minv[j] {
					minv[j] = cur
					way[j] = j0
				}
				if minv[j] < delta {
					delta = minv[j]
					j1 = j
				}
			}

			for j := 0; j <= n; j++ {
				if used[j] {
					u[p[j]] += delta
					v[j] -= delta
				} else {
					minv[j] -= delta
				}
			}

			j0 = j1
		}

		// Trace back augmenting path
		for j0 != 0 {
			p[j0] = p[way[j0]]
			j0 = way[j0]
		}
	}

	// Build 0-indexed assignments
	result := make([]Assignment, n)
	for j := 1; j <= n; j++ {
		if p[j] != 0 {
			result[p[j]-1] = Assignment{Row: p[j] - 1, Col: j - 1}
		}
	}
	return result
}
