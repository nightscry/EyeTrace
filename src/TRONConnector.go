package src

import (
	"encoding/hex"
	"errors"
	"fmt"
	"math"
)

//struct data Balance
type tronBalanceResponse struct{
	Data []struct{
		Balance int64 `json:"balance"`
	} `json:"data"`
}


// native txs: addresses come back as hex (41...), amount in sun, timestamp in ms
type tronTransactions struct{
	Data []struct{
		TxID string `json:"txID"`
		BlockTimestamp int64 `json:"block_timestamp"`
		Ret []struct{
			ContractRet string `json:"contractRet"`
		} `json:"ret"`
		RawData struct{
			Contract []struct{
				Type string `json:"type"`
				Parameter struct{
					Value struct{
						Amount int64 `json:"amount"`
						OwnerAddress string `json:"owner_address"`
						ToAddress string `json:"to_address"`
					} `json:"value"`
				} `json:"parameter"`
			} `json:"contract"`
		} `json:"raw_data"`
	} `json:"data"`
	Meta tronMeta `json:"meta"`
}

// TRC20 transfers: addresses already Base58, value is a decimal string like wei
type tronTRC20Transactions struct{
	Data []struct{
		TxID string `json:"transaction_id"`
		BlockTimestamp int64 `json:"block_timestamp"`
		From string `json:"from"`
		To string `json:"to"`
		Type string `json:"type"`
		Value string `json:"value"`
		TokenInfo struct{
			Address string `json:"address"`
			Symbol string `json:"symbol"`
			Decimals int `json:"decimals"`
		} `json:"token_info"`
	} `json:"data"`
	Meta tronMeta `json:"meta"`
}

// fingerprint is the cursor to the next page, empty on the last one
type tronMeta struct{
	Fingerprint string `json:"fingerprint"`
}

const tronPageMax = 200 // TronGrid rejects limit > 200

// Contracts of the real stablecoins. Anyone can deploy a token called "USDT",
// so the symbol alone proves nothing.
var tronKnownTokens = map[string]bool{
	"TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t": true, // USDT
	"TEkxiTehnzSmSe2XqrBj4w32RUN966rdz8": true, // USDC
}


//Get balance
func (T TronConnector)GetBalance(address string) (BalanceTxCount,error){
	url := fmt.Sprintf("https://api.trongrid.io/v1/accounts/%s", address)

	var result tronBalanceResponse
	if err := getJSON(url, &result); err != nil { return BalanceTxCount{}, err }

	// TronGrid answers {"success": false, "data": null} for an address it does not know:
	// indexing Data[0] here would panic inside the analysis goroutine and take the app down.
	if(len(result.Data) == 0){
		return BalanceTxCount{}, errors.New("address not found on Tron: " + address)
	}

	// balance is a JSON number in sun (1 TRX = 1e6 sun), not a decimal string like wei,
	// and sun fits an int64, so it needs no big.Int detour.
	// TODO: TxCount is still unset here; the accounts endpoint exposes no tx count.
	return BalanceTxCount{Balance: float64(result.Data[0].Balance) / 1e6}, nil

}


// Most Tron volume is USDT, so native TRX and TRC20 are fetched separately
// (up to limit each) and merged; Asset tells them apart.
func (T TronConnector)GetTransactions(address string, limit int) ([]Transfer,error) {
	if limit <= 0 {
		limit = 25
	}

	native, err := tronNative(address, limit)
	if err != nil { return nil, err }

	tokens, err := tronTRC20(address, limit)
	if err != nil { return nil, err }

	return append(native, tokens...), nil
}


func tronNative(address string, limit int) ([]Transfer, error) {
	var result []Transfer
	fetched := 0
	fingerprint := ""

	for fetched < limit {
		url := fmt.Sprintf("https://api.trongrid.io/v1/accounts/%s/transactions?only_confirmed=true&limit=%d", address, min(limit-fetched, tronPageMax))
		if fingerprint != "" {
			url += "&fingerprint=" + fingerprint
		}

		var page tronTransactions
		if err := getJSON(url, &page); err != nil { return nil, err }
		if len(page.Data) == 0 {
			break
		}

		for _, tx := range page.Data {
			fetched++

			// internal txs come without raw_data, and failed txs moved no funds
			if len(tx.RawData.Contract) == 0 || len(tx.Ret) == 0 || tx.Ret[0].ContractRet != "SUCCESS" {
				continue
			}
			c := tx.RawData.Contract[0]
			// smart contract calls (TRC20 included) are covered by tronTRC20
			if c.Type != "TransferContract" {
				continue
			}

			result = append(result, Transfer{
				From:      tronHexToBase58(c.Parameter.Value.OwnerAddress),
				To:        tronHexToBase58(c.Parameter.Value.ToAddress),
				Amount:    float64(c.Parameter.Value.Amount) / 1e6,
				Asset:     "TRX",
				Chain:     "TRON",
				Timestamp: tx.BlockTimestamp / 1000,
				Txhash:    tx.TxID,
			})
		}

		if page.Meta.Fingerprint == "" {
			break
		}
		fingerprint = page.Meta.Fingerprint
	}

	return result, nil
}


func tronTRC20(address string, limit int) ([]Transfer, error) {
	var result []Transfer
	fetched := 0
	fingerprint := ""

	for fetched < limit {
		url := fmt.Sprintf("https://api.trongrid.io/v1/accounts/%s/transactions/trc20?only_confirmed=true&limit=%d", address, min(limit-fetched, tronPageMax))
		if fingerprint != "" {
			url += "&fingerprint=" + fingerprint
		}

		var page tronTRC20Transactions
		if err := getJSON(url, &page); err != nil { return nil, err }
		if len(page.Data) == 0 {
			break
		}

		for _, tx := range page.Data {
			fetched++

			// the endpoint also lists Approval events, which move no funds
			if tx.Type != "Transfer" {
				continue
			}
			amount := ConversionBigToFloat(tx.Value, math.Pow10(tx.TokenInfo.Decimals))
			// zero-value transfers are the classic address-poisoning trick
			if amount == 0 {
				continue
			}

			asset := tx.TokenInfo.Symbol
			if !tronKnownTokens[tx.TokenInfo.Address] {
				asset += UnverifiedMark
			}
			result = append(result, Transfer{
				From:      tx.From,
				To:        tx.To,
				Amount:    amount,
				Asset:     asset,
				Chain:     "TRON",
				Timestamp: tx.BlockTimestamp / 1000,
				Txhash:    tx.TxID,
			})
		}

		if page.Meta.Fingerprint == "" {
			break
		}
		fingerprint = page.Meta.Fingerprint
	}

	return result, nil
}


// "41" + 20 bytes hex -> T... ; returns "" on garbage so aggregate() skips it
func tronHexToBase58(h string) string {
	raw, err := hex.DecodeString(h)
	if err != nil || len(raw) != 21 {
		return ""
	}
	return encodeBase58Check(raw, alphabetBase58)
}
