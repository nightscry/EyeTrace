package src

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

// An investigation is the target plus the history of every wallet expanded so far.
// It is what the graph is drawn from, and what gets saved and reopened, so a saved
// case can be reviewed offline without hitting the explorers again.
type Investigation struct {
	Version  string                `json:"version"`
	Created  int64                 `json:"created"`
	Target   string                `json:"target"`
	Chain    string                `json:"chain"`
	Testnet  bool                  `json:"testnet,omitempty"`
	Balance  float64               `json:"balance"`
	TxCount  int                   `json:"tx_count,omitempty"`
	Expanded []string              `json:"expanded"` // target first, then in expansion order
	History  map[string][]Transfer `json:"history"`
	Wallets  map[string]WalletInfo `json:"wallets,omitempty"` // recognition of target and expanded wallets
}

// What is known about a wallet that was analysed or expanded: the same recognition
// the target gets (detector subtype, balance, tx count, explorer-side identity).
type WalletInfo struct {
	Subtype    string  `json:"subtype,omitempty"`
	Balance    float64 `json:"balance"`
	HasBalance bool    `json:"has_balance"`           // false when the balance call failed
	BalanceErr string  `json:"balance_err,omitempty"` // why it failed, shown to the user
	TxCount    int     `json:"tx_count,omitempty"`
	Name       string  `json:"name,omitempty"` // on-chain name, e.g. a verified contract
	IsContract bool    `json:"is_contract,omitempty"`
}

func WalletInfoFrom(w Wallet, b BalanceTxCount, balanceErr error) WalletInfo {
	info := WalletInfo{
		Subtype: w.Subtype, Balance: b.Balance, HasBalance: balanceErr == nil,
		TxCount: b.TxCount, Name: b.Name, IsContract: b.IsContract,
	}
	if balanceErr != nil {
		info.BalanceErr = balanceErr.Error()
	}
	return info
}

// Recognition of any address, expanded or not: offline detector subtype, then the
// explorer's balance, tx count and identity. Costs one or two requests, no history.
func Recognize(c Connector, addr string) WalletInfo {
	w := Wallet{Address: addr}
	w.Indentification()
	b, err := c.GetBalance(addr)
	return WalletInfoFrom(w, b, err)
}

func NewInvestigation(w Wallet, b BalanceTxCount, transfers []Transfer) *Investigation {
	inv := &Investigation{
		Version: Version,
		Created: time.Now().Unix(),
		Target:  w.Address,
		Chain:   w.Type,
		Testnet: w.Testnet,
		Balance: b.Balance,
		TxCount: b.TxCount,
		History: map[string][]Transfer{},
		Wallets: map[string]WalletInfo{},
	}
	inv.AddHistory(w.Address, transfers)
	inv.SetWalletInfo(w.Address, WalletInfoFrom(w, b, nil))
	return inv
}

func (inv *Investigation) walletKey(addr string) string {
	if inv.Chain == "ETH" {
		return strings.ToLower(addr)
	}
	return addr
}

func (inv *Investigation) SetWalletInfo(addr string, info WalletInfo) {
	if inv.Wallets == nil {
		inv.Wallets = map[string]WalletInfo{}
	}
	inv.Wallets[inv.walletKey(addr)] = info
}

func (inv *Investigation) WalletInfo(addr string) (WalletInfo, bool) {
	info, ok := inv.Wallets[inv.walletKey(addr)]
	return info, ok
}

// History of an expanded wallet, whatever the case it was stored under.
func (inv *Investigation) HistoryOf(addr string) []Transfer {
	for _, a := range inv.Expanded {
		if SameAddress(inv.Chain, a, addr) {
			return inv.History[a]
		}
	}
	return nil
}

// EVM addresses compare case-insensitively, every other chain is case-sensitive.
func SameAddress(chain, a, b string) bool {
	if chain == "ETH" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

func (inv *Investigation) IsExpanded(addr string) bool {
	for _, a := range inv.Expanded {
		if SameAddress(inv.Chain, a, addr) {
			return true
		}
	}
	return false
}

func (inv *Investigation) AddHistory(addr string, transfers []Transfer) {
	if !inv.IsExpanded(addr) {
		inv.Expanded = append(inv.Expanded, addr)
	}
	inv.History[addr] = transfers
}

// Two expanded wallets that traded with each other both list the same transfers.
// A key seen n times in one history is kept n times (one BTC tx can pay the same
// address twice), but repeats across histories are dropped.
func (inv *Investigation) AllTransfers() []Transfer {
	kept := map[string]int{}
	var out []Transfer
	for _, addr := range inv.Expanded {
		seen := map[string]int{}
		for _, t := range inv.History[addr] {
			k := transferKey(inv.Chain, t)
			seen[k]++
			if seen[k] > kept[k] {
				kept[k] = seen[k]
				out = append(out, t)
			}
		}
	}
	return out
}

func transferKey(chain string, t Transfer) string {
	from, to := t.From, t.To
	if chain == "ETH" {
		from, to = strings.ToLower(from), strings.ToLower(to)
	}
	return fmt.Sprintf("%s|%s|%s|%s|%g", t.Txhash, from, to, t.Asset, t.Amount)
}

func (inv *Investigation) SaveJSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(inv)
}

func LoadInvestigation(r io.Reader) (*Investigation, error) {
	var inv Investigation
	if err := json.NewDecoder(r).Decode(&inv); err != nil {
		return nil, fmt.Errorf("not an EyeTrace investigation: %w", err)
	}
	if inv.Target == "" || inv.Chain == "" || len(inv.Expanded) == 0 {
		return nil, errors.New("not an EyeTrace investigation: target or chain missing")
	}
	if inv.History == nil {
		inv.History = map[string][]Transfer{}
	}
	// files saved before per-wallet info existed still carry the target's numbers
	if _, ok := inv.WalletInfo(inv.Target); !ok {
		inv.SetWalletInfo(inv.Target, WalletInfo{Balance: inv.Balance, HasBalance: true, TxCount: inv.TxCount})
	}
	return &inv, nil
}

// Flat table of every transfer, labels included, for spreadsheets and reports.
func (inv *Investigation) WriteCSV(w io.Writer) error {
	cw := csv.NewWriter(w)
	cw.Write([]string{"chain", "timestamp_utc", "txhash", "from", "from_label", "to", "to_label", "amount", "asset"})

	label := func(addr string) string {
		if e, ok := LookupEntity(inv.Chain, addr); ok {
			return e.Name
		}
		if info, ok := inv.WalletInfo(addr); ok {
			return info.Name
		}
		return ""
	}
	for _, t := range inv.AllTransfers() {
		ts := ""
		if t.Timestamp > 0 {
			ts = time.Unix(t.Timestamp, 0).UTC().Format(time.RFC3339)
		}
		cw.Write([]string{
			t.Chain, ts, t.Txhash,
			t.From, label(t.From), t.To, label(t.To),
			strconv.FormatFloat(t.Amount, 'f', -1, 64), t.Asset,
		})
	}
	cw.Flush()
	return cw.Error()
}
