package loadtest

import "testing"

func TestScheduleBalancesClientsWithoutAdjacentRepeats(t *testing.T) {
	for _, total := range []int{0, 1, 4, 17, 4099} {
		plan := ClientSchedule(total, 4)
		if len(plan) != total {
			t.Fatal("wrong request count")
		}
		counts := make([]int, 4)
		for seq, client := range plan {
			if client < 0 || client >= 4 {
				t.Fatal("unknown client")
			}
			counts[client]++
			if seq > 0 && plan[seq-1] == client {
				t.Fatal("adjacent client repeated")
			}
		}
		for offset := 0; offset+4 <= total; offset += 4 {
			seen := map[int]bool{}
			for _, client := range plan[offset : offset+4] {
				seen[client] = true
			}
			if len(seen) != 4 {
				t.Fatal("missing client in complete group")
			}
		}
		for _, count := range counts {
			if count < total/4 || count > (total+3)/4 {
				t.Fatal("unbalanced client distribution")
			}
		}
	}
}
