package src

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
)

// Public XRPL JSON-RPC node, keyless (s1.ripple.com:51234 speaks the same API).
const xrplURL = "https://xrplcluster.com"

// XRPL timestamps count seconds from 2000-01-01, not from 1970.
const rippleEpoch = 946684800

type xrplRequest struct {
	Method string `json:"method"`
	Params []any  `json:"params"`
}

// common to every response: errors come back as HTTP 200 with status "error"
type xrplStatus struct {
	Status       string `json:"status"`
	Error        string `json:"error"`
	ErrorMessage string `json:"error_message"`
}

func (s xrplStatus) err() error {
	if s.Status == "success" {
		return nil
	}
	if s.Error == "actNotFound" {
		return errors.New("account not found on XRPL (never activated)")
	}
	if s.ErrorMessage != "" {
		return errors.New("XRPL: " + s.ErrorMessage)
	}
	return errors.New("XRPL: " + s.Error)
}

type xrplAccountInfo struct {
	Result struct {
		xrplStatus
		AccountData struct {
			Balance string `json:"Balance"` // drops, as a string
		} `json:"account_data"`
	} `json:"result"`
}

type xrplAccountTx struct {
	Result struct {
		xrplStatus
		Marker       json.RawMessage `json:"marker"` // opaque cursor, sent back as is
		Transactions []struct {
			Tx struct {
				TransactionType string          `json:"TransactionType"`
				Account         string          `json:"Account"`
				Destination     string          `json:"Destination"`
				Amount          json.RawMessage `json:"Amount"`
				Date            int64           `json:"date"`
				Hash            string          `json:"hash"`
			} `json:"tx"`
			Meta struct {
				TransactionResult string          `json:"TransactionResult"`
				DeliveredAmount   json.RawMessage `json:"delivered_amount"`
			} `json:"meta"`
		} `json:"transactions"`
	} `json:"result"`
}

// Issued currencies are only as good as their issuer: "USD" can be minted by anyone.
// Keyed issuer + currency; RLUSD's issuer carries the ripple.com domain on-ledger.
var xrplKnownIssued = map[string]bool{
	"rMxCKbEDwqr76QuheSUMdEGf4B9xJ8m5De:RLUSD": true,
}

const xrplPageMax = 400 // account_tx refuses bigger pages

func (X XRPConnector) GetBalance(address string) (BalanceTxCount, error) {
	var resp xrplAccountInfo
	req := xrplRequest{Method: "account_info", Params: []any{map[string]any{
		"account": address, "ledger_index": "validated",
	}}}
	if err := postJSON(xrplURL, req, &resp); err != nil {
		return BalanceTxCount{}, err
	}
	if err := resp.Result.err(); err != nil {
		return BalanceTxCount{}, err
	}

	drops, _ := strconv.ParseInt(resp.Result.AccountData.Balance, 10, 64)
	// TODO: TxCount stays 0, account_info exposes only the sender sequence
	return BalanceTxCount{Balance: float64(drops) / 1e6}, nil
}

func (X XRPConnector) GetTransactions(address string, limit int) ([]Transfer, error) {
	if limit <= 0 {
		limit = 25
	}

	var result []Transfer
	fetched := 0
	var marker json.RawMessage

	for fetched < limit {
		params := map[string]any{"account": address, "limit": min(limit-fetched, xrplPageMax)}
		if marker != nil {
			params["marker"] = marker
		}

		var resp xrplAccountTx
		if err := postJSON(xrplURL, xrplRequest{Method: "account_tx", Params: []any{params}}, &resp); err != nil {
			return nil, err
		}
		if err := resp.Result.err(); err != nil {
			return nil, err
		}
		if len(resp.Result.Transactions) == 0 {
			break
		}

		for _, t := range resp.Result.Transactions {
			fetched++
			if t.Tx.TransactionType != "Payment" || t.Meta.TransactionResult != "tesSUCCESS" {
				continue
			}

			// delivered_amount, not Amount: with a partial payment Amount is only the
			// ceiling, and faking big incoming payments that way is a known scam
			raw := t.Meta.DeliveredAmount
			if len(raw) == 0 || string(raw) == `"unavailable"` {
				raw = t.Tx.Amount
			}
			amount, asset, ok := xrplAmount(raw)
			if !ok || amount == 0 {
				continue
			}

			result = append(result, Transfer{
				From:      t.Tx.Account,
				To:        t.Tx.Destination,
				Amount:    amount,
				Asset:     asset,
				Chain:     "XRP",
				Timestamp: t.Tx.Date + rippleEpoch,
				Txhash:    t.Tx.Hash,
			})
		}

		if len(resp.Result.Marker) == 0 || string(resp.Result.Marker) == "null" {
			break
		}
		marker = resp.Result.Marker
	}

	return result, nil
}

// An amount is either a string of XRP drops or an object for an issued currency.
func xrplAmount(raw json.RawMessage) (float64, string, bool) {
	var drops string
	if err := json.Unmarshal(raw, &drops); err == nil {
		d, err := strconv.ParseInt(drops, 10, 64)
		return float64(d) / 1e6, "XRP", err == nil
	}

	var issued struct {
		Currency string `json:"currency"`
		Issuer   string `json:"issuer"`
		Value    string `json:"value"`
	}
	if err := json.Unmarshal(raw, &issued); err != nil {
		return 0, "", false
	}
	v, err := strconv.ParseFloat(issued.Value, 64)
	if err != nil {
		return 0, "", false
	}

	code := xrplCurrencyCode(issued.Currency)
	if !xrplKnownIssued[issued.Issuer+":"+code] {
		code += UnverifiedMark
	}
	return v, code, true
}

// Codes longer than 3 chars travel as 40 hex digits (160 bits), ASCII padded with zeros.
func xrplCurrencyCode(c string) string {
	if len(c) != 40 {
		return c
	}
	b, err := hex.DecodeString(c)
	if err != nil {
		return c
	}
	s := strings.TrimRight(string(b), "\x00")
	for _, r := range s {
		if r < 0x20 || r > 0x7e {
			return c[:8] // not text: keep a short hex tag
		}
	}
	return s
}
