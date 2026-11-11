package src

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"time"
)

// Public mainnet RPC, keyless but tightly rate-limited: that is why transactions
// are fetched in JSON-RPC batches and the batches are spaced out.
const solanaURL = "https://api.mainnet-beta.solana.com"

const (
	solanaSigPageMax = 1000 // getSignaturesForAddress cap
	solanaBatch      = 20   // getTransaction calls per HTTP request
	solanaBatchPause = 500 * time.Millisecond
)

// Mints of the main SPL tokens. Anyone can mint a token named "USDC",
// so the mint address is what identifies it.
var solanaKnownMints = map[string]string{
	"EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v": "USDC",
	"Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB": "USDT",
	"So11111111111111111111111111111111111111112":  "wSOL",
}

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Method  string `json:"method"`
	Params  []any  `json:"params"`
}

type rpcResponse struct {
	ID     int             `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func solanaCall(method string, params []any, out any) error {
	var resp rpcResponse
	if err := postJSON(solanaURL, rpcRequest{"2.0", 1, method, params}, &resp); err != nil {
		return err
	}
	if resp.Error != nil {
		return fmt.Errorf("Solana RPC: %s", resp.Error.Message)
	}
	return json.Unmarshal(resp.Result, out)
}

type solanaSignature struct {
	Signature string `json:"signature"`
	Err       any    `json:"err"`
}

type solanaInstruction struct {
	Program string          `json:"program"`
	Parsed  json.RawMessage `json:"parsed"` // an object for system transfers, a string or absent elsewhere
}

type solanaTokenBalance struct {
	AccountIndex  int    `json:"accountIndex"`
	Mint          string `json:"mint"`
	Owner         string `json:"owner"`
	UITokenAmount struct {
		Amount   string `json:"amount"`
		Decimals int    `json:"decimals"`
	} `json:"uiTokenAmount"`
}

type solanaTx struct {
	BlockTime int64 `json:"blockTime"`
	Meta      struct {
		Err               any                  `json:"err"`
		PreTokenBalances  []solanaTokenBalance `json:"preTokenBalances"`
		PostTokenBalances []solanaTokenBalance `json:"postTokenBalances"`
		InnerInstructions []struct {
			Instructions []solanaInstruction `json:"instructions"`
		} `json:"innerInstructions"`
	} `json:"meta"`
	Transaction struct {
		Signatures []string `json:"signatures"`
		Message    struct {
			Instructions []solanaInstruction `json:"instructions"`
		} `json:"message"`
	} `json:"transaction"`
}

func (S SolanaConnector) GetBalance(address string) (BalanceTxCount, error) {
	var res struct {
		Value uint64 `json:"value"` // lamports, 1 SOL = 1e9
	}
	if err := solanaCall("getBalance", []any{address}, &res); err != nil {
		return BalanceTxCount{}, err
	}
	// TODO: TxCount stays 0, the RPC has no counter short of paging every signature
	return BalanceTxCount{Balance: float64(res.Value) / 1e9}, nil
}

// Two steps: signatures first (cheap, paged with "before"), then the full
// transactions, which carry the actual movements.
func (S SolanaConnector) GetTransactions(address string, limit int) ([]Transfer, error) {
	if limit <= 0 {
		limit = 25
	}

	var sigs []string
	before := ""
	for len(sigs) < limit {
		opts := map[string]any{"limit": min(limit-len(sigs), solanaSigPageMax)}
		if before != "" {
			opts["before"] = before
		}
		var page []solanaSignature
		if err := solanaCall("getSignaturesForAddress", []any{address, opts}, &page); err != nil {
			return nil, err
		}
		if len(page) == 0 {
			break
		}
		for _, s := range page {
			if s.Err == nil { // failed txs moved nothing
				sigs = append(sigs, s.Signature)
			}
		}
		before = page[len(page)-1].Signature
		if len(page) < solanaSigPageMax {
			break
		}
	}

	var result []Transfer
	for start := 0; start < len(sigs); start += solanaBatch {
		if start > 0 {
			time.Sleep(solanaBatchPause)
		}
		txs, err := solanaFetchBatch(sigs[start:min(start+solanaBatch, len(sigs))])
		if err != nil {
			return nil, err
		}
		for _, tx := range txs {
			result = append(result, solanaTransfers(address, tx)...)
		}
	}
	return result, nil
}

func solanaFetchBatch(sigs []string) ([]solanaTx, error) {
	reqs := make([]rpcRequest, len(sigs))
	for i, s := range sigs {
		reqs[i] = rpcRequest{"2.0", i, "getTransaction", []any{s, map[string]any{
			"encoding": "jsonParsed", "maxSupportedTransactionVersion": 0,
		}}}
	}

	var resps []rpcResponse
	if err := postJSON(solanaURL, reqs, &resps); err != nil {
		return nil, err
	}

	var txs []solanaTx
	var lastErr error
	for _, r := range resps {
		if r.Error != nil {
			lastErr = errors.New("Solana RPC: " + r.Error.Message)
			continue
		}
		var tx solanaTx
		// a null result is a tx the node no longer holds
		if err := json.Unmarshal(r.Result, &tx); err != nil || tx.Transaction.Signatures == nil {
			continue
		}
		txs = append(txs, tx)
	}
	// a few failed items are tolerated, a fully failed batch is not
	if len(txs) == 0 && lastErr != nil {
		return nil, lastErr
	}
	return txs, nil
}

// SOL moves through System Program transfer instructions (top-level or inner, when a
// program pays out). SPL tokens are read from the balance changes instead: transfer
// instructions name token accounts, not wallets, while token balances carry the owner.
func solanaTransfers(target string, tx solanaTx) []Transfer {
	if tx.Meta.Err != nil || len(tx.Transaction.Signatures) == 0 {
		return nil
	}
	hash := tx.Transaction.Signatures[0]
	var out []Transfer

	instructions := tx.Transaction.Message.Instructions
	for _, inner := range tx.Meta.InnerInstructions {
		instructions = append(instructions, inner.Instructions...)
	}
	for _, ins := range instructions {
		if ins.Program != "system" {
			continue
		}
		var p struct {
			Type string `json:"type"`
			Info struct {
				Source      string `json:"source"`
				Destination string `json:"destination"`
				Lamports    uint64 `json:"lamports"`
			} `json:"info"`
		}
		if json.Unmarshal(ins.Parsed, &p) != nil || (p.Type != "transfer" && p.Type != "transferWithSeed") {
			continue
		}
		if p.Info.Lamports == 0 || p.Info.Source == p.Info.Destination {
			continue
		}
		out = append(out, Transfer{
			From: p.Info.Source, To: p.Info.Destination,
			Amount: float64(p.Info.Lamports) / 1e9, Asset: "SOL",
			Chain: "SOLANA", Timestamp: tx.BlockTime, Txhash: hash,
		})
	}

	return append(out, solanaTokenTransfers(target, tx, hash)...)
}

type ownerMint struct{ owner, mint string }

func solanaTokenTransfers(target string, tx solanaTx, hash string) []Transfer {
	// net change per token account, then summed per (owner, mint)
	type acct struct {
		owner, mint string
		decimals    int
		pre, post   *big.Int
	}
	accts := map[int]*acct{}
	get := func(b solanaTokenBalance) *acct {
		a := accts[b.AccountIndex]
		if a == nil {
			a = &acct{owner: b.Owner, mint: b.Mint, decimals: b.UITokenAmount.Decimals, pre: new(big.Int), post: new(big.Int)}
			accts[b.AccountIndex] = a
		}
		return a
	}
	for _, b := range tx.Meta.PreTokenBalances {
		get(b).pre.SetString(b.UITokenAmount.Amount, 10)
	}
	for _, b := range tx.Meta.PostTokenBalances {
		get(b).post.SetString(b.UITokenAmount.Amount, 10)
	}

	delta := map[ownerMint]*big.Int{}
	decimals := map[string]int{}
	for _, a := range accts {
		if a.owner == "" {
			continue
		}
		k := ownerMint{a.owner, a.mint}
		if delta[k] == nil {
			delta[k] = new(big.Int)
		}
		delta[k].Add(delta[k], new(big.Int).Sub(a.post, a.pre))
		decimals[a.mint] = a.decimals
	}

	var out []Transfer
	for k, d := range delta {
		if k.owner != target || d.Sign() == 0 {
			continue
		}
		// counterparties are the owners that moved the same mint the other way;
		// each flow is capped at what the target itself moved
		for k2, d2 := range delta {
			if k2.mint != k.mint || k2.owner == target || d2.Sign() == 0 || d2.Sign() == d.Sign() {
				continue
			}
			amt := new(big.Int).Abs(d2)
			if abs := new(big.Int).Abs(d); amt.Cmp(abs) > 0 {
				amt = abs
			}
			from, to := target, k2.owner
			if d.Sign() > 0 {
				from, to = k2.owner, target
			}
			out = append(out, Transfer{
				From: from, To: to,
				Amount: ConversionBigToFloat(amt.String(), math.Pow10(decimals[k.mint])),
				Asset:  solanaAsset(k.mint),
				Chain:  "SOLANA", Timestamp: tx.BlockTime, Txhash: hash,
			})
		}
	}
	return out
}

func solanaAsset(mint string) string {
	if s, ok := solanaKnownMints[mint]; ok {
		return s
	}
	if len(mint) > 6 {
		mint = mint[:6]
	}
	return mint + "…" + UnverifiedMark // no symbol on-chain without the metadata program
}
