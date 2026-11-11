package src

import (
	"fmt"
	"sort"
)

// Heuristic recognition for wallets that no label list knows. Exchanges, payment
// processors and bots show up as sheer volume: a lifetime tx count in the hundreds
// of thousands, or a fetched page of history that spans only hours. These are hints,
// not identities, and the GUI presents them as such.
const (
	serviceTxCount   = 100_000 // lifetime txs
	busyMinTxs       = 50      // distinct txs needed before judging the rate
	busyTxsPerDayMin = 200
)

type Profile struct {
	ServiceLike bool     // volume typical of an exchange or service
	Hints       []string // human-readable reasons, plus neutral facts
}

func ProfileOf(info WalletInfo, history []Transfer) Profile {
	var p Profile

	if info.IsContract {
		p.Hints = append(p.Hints, "Smart contract, not a person's wallet")
	}
	if info.TxCount >= serviceTxCount {
		p.ServiceLike = true
		p.Hints = append(p.Hints, fmt.Sprintf("%d lifetime transactions: volume typical of an exchange or service", info.TxCount))
	}

	// rate over the fetched window: distinct txs per day between oldest and newest
	seen := map[string]bool{}
	var times []int64
	for _, t := range history {
		if seen[t.Txhash] || t.Timestamp == 0 {
			continue
		}
		seen[t.Txhash] = true
		times = append(times, t.Timestamp)
	}
	if len(times) >= busyMinTxs {
		sort.Slice(times, func(i, j int) bool { return times[i] < times[j] })
		days := float64(times[len(times)-1]-times[0]) / 86400
		if days < 1.0/24 {
			days = 1.0 / 24 // an hour at least, so a burst does not divide by ~0
		}
		rate := float64(len(times)) / days
		if rate >= busyTxsPerDayMin {
			p.ServiceLike = true
			p.Hints = append(p.Hints, fmt.Sprintf("~%.0f transactions/day in the fetched window: automated or service-like activity", rate))
		}
	}
	return p
}
