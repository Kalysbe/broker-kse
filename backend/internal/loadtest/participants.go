package loadtest

import "math/rand/v2"

type Participant struct {
	AccountID  string `json:"account_id"`
	ClientCode string `json:"client_code"`
}

// Precompute assignments before workers start. Each complete group contains
// every participant once, shuffled, without adjacent repeats across groups.
func ClientSchedule(total, clients int) []int {
	if clients < 1 {
		return nil
	}
	plan := make([]int, total)
	last := -1
	for offset := 0; offset < total; offset += clients {
		group := make([]int, clients)
		for i := range group {
			group[i] = i
		}
		for i := clients - 1; i > 0; i-- {
			j := rand.IntN(i + 1)
			group[i], group[j] = group[j], group[i]
		}
		if clients > 1 && group[0] == last {
			j := 1 + rand.IntN(clients-1)
			group[0], group[j] = group[j], group[0]
		}
		n := min(clients, total-offset)
		copy(plan[offset:offset+n], group[:n])
		last = group[n-1]
	}
	return plan
}
