package gui

import (
	"fmt"
	"math"
	"testing"

	"eyetracker/src"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
)

const tornado10 = "0x910Cbd523D972eb0a6f4cAe4618aD62622b39DbF"

func tr(from, to string, amount float64, asset string) src.Transfer {
	return src.Transfer{From: from, To: to, Amount: amount, Asset: asset, Txhash: from + to, Chain: "ETH"}
}

func TestAggregateDirectionsAndAssets(t *testing.T) {
	txs := []src.Transfer{
		tr("0xT", "0xa", 5, "ETH"),
		tr("0xA", "0xt", 2, "ETH"), // case differs: still the same pair
		tr("0xT", "0xb", 1, "ETH"),
		tr("0xT", "0xa", 999, "USDT"), // other asset: ignored
		tr("0xT", "0xT", 3, "ETH"),    // self transfer: ignored
		tr("", "0xT", 3, "ETH"),       // coinbase-like: ignored
	}
	got := aggregate("0xT", "ETH", txs, 10)
	if len(got) != 2 {
		t.Fatalf("want 2 counterparties, got %+v", got)
	}
	a := got[0]
	if a.sent != 5 || a.recv != 2 || a.count != 2 {
		t.Errorf("0xa: %+v", a)
	}
	if len(aggregate("0xT", "ETH", txs, 1)) != 1 {
		t.Error("maxNodes not applied")
	}
}

func TestAssetsOfOrder(t *testing.T) {
	txs := []src.Transfer{
		tr("a", "b", 1, "SPAM?"), tr("a", "b", 1, "SPAM?"), tr("a", "b", 1, "SPAM?"),
		tr("a", "b", 1, "USDT"),
		tr("a", "b", 1, "TRX"),
	}
	got := assetsOf(txs, "TRX")
	// unverified last despite being the most frequent; native wins the tie
	if got[0].Asset != "TRX" || got[1].Asset != "USDT" || got[2].Asset != "SPAM?" {
		t.Fatalf("order: %+v", got)
	}
}

func TestBuildGraphMultiHop(t *testing.T) {
	target := "0xT"
	inv := src.NewInvestigation(src.Wallet{Address: target, Type: "ETH"}, src.BalanceTxCount{Balance: 7, TxCount: 3},
		[]src.Transfer{tr(target, "0xa", 5, "ETH"), tr("0xb", target, 1, "ETH")})
	// 0xa's own history: the same tx with the target, plus a payment into a mixer
	inv.AddHistory("0xa", []src.Transfer{tr("0xt", "0xA", 5, "ETH"), tr("0xA", tornado10, 4, "ETH")})

	g := newGraphWidget(inv, "ETH", 10)
	if len(g.nodes) != 4 {
		t.Fatalf("want target, 0xa, 0xb, mixer: got %d nodes", len(g.nodes))
	}
	if len(g.edges) != 3 {
		t.Fatalf("T-a must be built once: got %d edges", len(g.edges))
	}
	if !g.nodes[g.indexOf("0xa")].expanded {
		t.Error("0xa should be marked expanded")
	}

	info := g.infoOf(0)
	if info.Out != 5 || info.In != 1 || info.Links != 2 || info.Balance != 7 || info.TxCount != 3 {
		t.Errorf("target info: %+v", info)
	}
	if len(info.Risk) != 1 || info.Risk[0] != "2 hops: Tornado Cash 10 ETH" {
		t.Errorf("risk: %v", info.Risk)
	}

	a := g.infoOf(g.indexOf("0xa"))
	if a.In != 5 || a.Out != 4 || a.Links != 2 {
		t.Errorf("0xa info: %+v", a)
	}
}

func TestAmountFormat(t *testing.T) {
	cases := map[float64]string{
		0.00012345:   "0.00012345 BTC",
		1234.5:       "1,234.5000 BTC",
		608918774.76: "608,918,775 BTC",
	}
	for v, want := range cases {
		if got := amount(v, "BTC"); got != want {
			t.Errorf("amount(%v) = %q, want %q", v, got, want)
		}
	}
}

// An expanded wallet must move away from the target and take its own star with it,
// instead of staying where it sat as a leaf.
func TestExpandedWalletSeparates(t *testing.T) {
	test.NewTempApp(t) // Render starts the graph animation, which needs a running app
	target := "0xT"
	var first []src.Transfer
	for i := 0; i < 10; i++ {
		first = append(first, tr(target, fmt.Sprintf("0xleaf%d", i), 1, "ETH"))
	}
	inv := src.NewInvestigation(src.Wallet{Address: target, Type: "ETH"}, src.BalanceTxCount{}, first)

	old := newGraphWidget(inv, "ETH", 25)
	old.size = fyne.NewSize(800, 600)
	for i := 0; i < 200; i++ {
		old.step()
	}

	var second []src.Transfer
	for i := 0; i < 10; i++ {
		second = append(second, tr("0xleaf0", fmt.Sprintf("0xfar%d", i), 1, "ETH"))
	}
	inv.AddHistory("0xleaf0", second)

	gv := &GraphView{host: container.NewStack(), current: old}
	gv.Render(inv, "ETH", 25)
	g := gv.current
	defer gv.stop()
	g.size = old.size
	for i := 0; i < 400; i++ {
		g.step()
	}

	hub := g.nodes[g.indexOf("0xleaf0")]
	if d := math.Hypot(float64(hub.x), float64(hub.y)); d < float64(simHubLen)*0.7 {
		t.Errorf("expanded wallet still %.0f from the target, want about %.0f", d, simHubLen)
	}
	// its counterparties sit around it, not around the target
	closer := 0
	for i := 0; i < 10; i++ {
		n := g.nodes[g.indexOf(fmt.Sprintf("0xfar%d", i))]
		if math.Hypot(float64(n.x-hub.x), float64(n.y-hub.y)) < math.Hypot(float64(n.x), float64(n.y)) {
			closer++
		}
	}
	if closer < 8 { // random start: 8/10 in about 1 run out of 8, never fewer
		t.Errorf("only %d/10 of the new counterparties are closer to their wallet than to the target", closer)
	}
}

func TestExpandedWalletRecognized(t *testing.T) {
	target := "0xT"
	inv := src.NewInvestigation(src.Wallet{Address: target, Type: "ETH", Subtype: "EIP-55"}, src.BalanceTxCount{Balance: 1},
		[]src.Transfer{tr(target, "0xpool", 1, "ETH")})
	inv.AddHistory("0xpool", nil)
	inv.SetWalletInfo("0xPOOL", src.WalletInfo{Balance: 50, HasBalance: true, TxCount: 250000, Name: "SomeRouter", IsContract: true})

	g := newGraphWidget(inv, "ETH", 10)
	i := g.indexOf("0xpool")
	n := g.nodes[i]
	if n.label != "SomeRouter" || !n.contract || !n.service {
		t.Fatalf("node: label=%q contract=%v service=%v", n.label, n.contract, n.service)
	}
	info := g.infoOf(i)
	if !info.Recognized || info.Balance != 50 || info.TxCount != 250000 || !info.Profile.ServiceLike {
		t.Fatalf("info: %+v", info)
	}
	if !g.infoOf(0).Recognized || g.infoOf(0).Subtype != "EIP-55" {
		t.Fatal("target must stay recognised")
	}
}

// a counterparty looked up on click gets its name and type without being expanded
func TestCounterpartyLookupUpdatesNode(t *testing.T) {
	test.NewTempApp(t)
	target := "0xT"
	inv := src.NewInvestigation(src.Wallet{Address: target, Type: "ETH"}, src.BalanceTxCount{},
		[]src.Transfer{tr(target, "0xrouter", 1, "ETH")})
	gv := &GraphView{host: container.NewStack()}
	gv.Render(inv, "ETH", 10)
	defer gv.stop()

	inv.SetWalletInfo("0xRouter", src.WalletInfo{Subtype: "EIP-55", HasBalance: true, Name: "SomeRouter", IsContract: true})
	gv.UpdateWallet("0xrouter")
	i := gv.current.indexOf("0xrouter")
	n := gv.current.nodes[i]
	if n.label != "SomeRouter" || !n.contract || n.expanded {
		t.Fatalf("node: %+v", n)
	}
	if info := gv.current.infoOf(i); !info.Recognized || info.Subtype != "EIP-55" || info.Expanded {
		t.Fatalf("info: %+v", info)
	}
}
