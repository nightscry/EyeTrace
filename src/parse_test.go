package src

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestXrplAmount(t *testing.T) {
	cases := []struct {
		raw    string
		amount float64
		asset  string
	}{
		{`"9899000"`, 9.899, "XRP"},
		{`{"currency":"USD","issuer":"rSomeone","value":"12.5"}`, 12.5, "USD?"},
		{`{"currency":"524C555344000000000000000000000000000000","issuer":"rMxCKbEDwqr76QuheSUMdEGf4B9xJ8m5De","value":"3"}`, 3, "RLUSD"},
		// same code from another issuer is not the real RLUSD
		{`{"currency":"524C555344000000000000000000000000000000","issuer":"rFake","value":"3"}`, 3, "RLUSD?"},
	}
	for _, c := range cases {
		amount, asset, ok := xrplAmount(json.RawMessage(c.raw))
		if !ok || amount != c.amount || asset != c.asset {
			t.Errorf("%s: got %v %q %v", c.raw, amount, asset, ok)
		}
	}
}

func TestSolanaTransfers(t *testing.T) {
	const target = "Target1111111111111111111111111111111111111"
	raw := `{
	  "blockTime": 1700000000,
	  "meta": {
	    "err": null,
	    "preTokenBalances": [
	      {"accountIndex": 1, "mint": "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v", "owner": "` + target + `", "uiTokenAmount": {"amount": "5000000", "decimals": 6}},
	      {"accountIndex": 2, "mint": "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v", "owner": "Bob", "uiTokenAmount": {"amount": "0", "decimals": 6}}
	    ],
	    "postTokenBalances": [
	      {"accountIndex": 1, "mint": "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v", "owner": "` + target + `", "uiTokenAmount": {"amount": "3000000", "decimals": 6}},
	      {"accountIndex": 2, "mint": "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v", "owner": "Bob", "uiTokenAmount": {"amount": "2000000", "decimals": 6}}
	    ],
	    "innerInstructions": [
	      {"instructions": [{"program": "system", "parsed": {"type": "transfer", "info": {"source": "Alice", "destination": "` + target + `", "lamports": 1500000000}}}]}
	    ]
	  },
	  "transaction": {
	    "signatures": ["sig1"],
	    "message": {"instructions": [
	      {"program": "spl-memo", "parsed": "hello"},
	      {"program": "system", "parsed": {"type": "createAccount", "info": {"source": "X", "newAccount": "Y", "lamports": 1}}}
	    ]}
	  }
	}`
	var tx solanaTx
	if err := json.Unmarshal([]byte(raw), &tx); err != nil {
		t.Fatal(err)
	}
	got := solanaTransfers(target, tx)
	if len(got) != 2 {
		t.Fatalf("want 2 transfers (1 SOL inner + 1 USDC), got %+v", got)
	}
	sol, usdc := got[0], got[1]
	if sol.From != "Alice" || sol.To != target || sol.Amount != 1.5 || sol.Asset != "SOL" {
		t.Errorf("SOL transfer: %+v", sol)
	}
	if usdc.From != target || usdc.To != "Bob" || usdc.Amount != 2 || usdc.Asset != "USDC" || usdc.Txhash != "sig1" {
		t.Errorf("USDC transfer: %+v", usdc)
	}
}

func TestInvestigationDedupAndRoundTrip(t *testing.T) {
	a2b := Transfer{From: "0xAAA", To: "0xbbb", Amount: 1, Asset: "ETH", Txhash: "h1", Chain: "ETH"}
	inv := NewInvestigation(Wallet{Address: "0xAAA", Type: "ETH"}, BalanceTxCount{Balance: 2}, []Transfer{a2b, a2b})
	// the counterparty's own history lists the same tx, lowercased by the explorer
	inv.AddHistory("0xbbb", []Transfer{{From: "0xaaa", To: "0xbbb", Amount: 1, Asset: "ETH", Txhash: "h1", Chain: "ETH"}})

	if !inv.IsExpanded("0xaaa") || !inv.IsExpanded("0xBBB") {
		t.Fatal("ETH expansion lookup must ignore case")
	}
	// kept twice (two outputs in one history), not three times
	if n := len(inv.AllTransfers()); n != 2 {
		t.Fatalf("AllTransfers = %d, want 2", n)
	}

	var buf bytes.Buffer
	if err := inv.SaveJSON(&buf); err != nil {
		t.Fatal(err)
	}
	back, err := LoadInvestigation(&buf)
	if err != nil || back.Target != "0xAAA" || len(back.Expanded) != 2 || back.Balance != 2 {
		t.Fatalf("round trip: %+v %v", back, err)
	}

	if _, err := LoadInvestigation(strings.NewReader(`{"foo": 1}`)); err == nil {
		t.Fatal("a random JSON must be refused")
	}

	buf.Reset()
	if err := inv.WriteCSV(&buf); err != nil {
		t.Fatal(err)
	}
	if lines := strings.Count(buf.String(), "\n"); lines != 3 { // header + 2 rows
		t.Fatalf("CSV has %d lines:\n%s", lines, buf.String())
	}
}

func TestLookupEntity(t *testing.T) {
	e, ok := LookupEntity("ETH", "0x910Cbd523D972eb0a6f4cAe4618aD62622b39DbF")
	if !ok || !e.Risky() {
		t.Fatalf("checksummed Tornado address must match: %+v %v", e, ok)
	}
	if _, ok := LookupEntity("TRON", "tv6mumxfmlbbqpzvbhdwfsdnqevfnmiusi"); ok {
		t.Fatal("non-EVM lookups are case-sensitive")
	}
}

func TestProfileHeuristic(t *testing.T) {
	if p := ProfileOf(WalletInfo{TxCount: 5}, nil); p.ServiceLike || len(p.Hints) != 0 {
		t.Fatalf("quiet wallet: %+v", p)
	}
	if p := ProfileOf(WalletInfo{TxCount: 150000}, nil); !p.ServiceLike {
		t.Fatal("150k lifetime txs must look service-like")
	}
	// 60 distinct txs within one hour
	var busy []Transfer
	for i := 0; i < 60; i++ {
		busy = append(busy, Transfer{Txhash: strings.Repeat("a", i+1), Timestamp: 1700000000 + int64(i*60)})
	}
	if p := ProfileOf(WalletInfo{}, busy); !p.ServiceLike {
		t.Fatalf("60 tx/hour must look service-like: %+v", p)
	}
	// same 60 txs spread over 60 days: an ordinary wallet
	for i := range busy {
		busy[i].Timestamp = 1700000000 + int64(i*86400)
	}
	if p := ProfileOf(WalletInfo{}, busy); p.ServiceLike {
		t.Fatalf("1 tx/day is not service-like: %+v", p)
	}
}

func TestLoadOldInvestigationKeepsTargetInfo(t *testing.T) {
	old := `{"version":"1.0.0","target":"T1","chain":"TRON","balance":12,"tx_count":3,"expanded":["T1"],"history":{"T1":[]}}`
	inv, err := LoadInvestigation(strings.NewReader(old))
	if err != nil {
		t.Fatal(err)
	}
	if info, ok := inv.WalletInfo("T1"); !ok || info.Balance != 12 || info.TxCount != 3 {
		t.Fatalf("target info from an older file: %+v %v", info, ok)
	}
}
