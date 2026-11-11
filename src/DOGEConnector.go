package src

import (
	"fmt"
	"time"
)

type addressResponseDoge struct{
	Balance float64 `json:"final_balance"`
	Tx int `json:"n_tx"`
}

type transactionDOGE struct{
	Txs []struct{
		Inputs []struct{
			Addresses []string `json:"addresses"`
		}`json:"inputs"`
		Output []struct{
			Addresses []string `json:"addresses"`
			Value int64 `json:"value"`
		} `json:"outputs"`
		Hash string `json:"hash"`
		Confirmed string `json:"confirmed"`
		BlockHeight int `json:"block_height"`
	} `json:"txs"`
}


func (C DOGEConnector)GetBalance(address string)(BalanceTxCount,error){
	url := fmt.Sprintf("https://api.blockcypher.com/v1/doge/main/addrs/%s/balance", address)

	var data addressResponseDoge
	if err := getJSON(url, &data); err != nil {return BalanceTxCount{}, err}

	result := BalanceTxCount{
		Balance: float64(data.Balance) / 100_000_000,
		TxCount: data.Tx,
	}

	return result, nil
}


// BlockCypher /full caps out around 50 txs per call; before=<block_height> walks back
// to older blocks. Unlike Blockstream a short page is NOT the end of history, because
// BlockCypher truncates on response size, so we only stop on an empty page, on the
// limit, or when the cursor stops going down.
func (C DOGEConnector)GetTransactions(address string, limit int)([]Transfer,error){
	if limit <= 0 {
		limit = 25
	}

	base := "https://api.blockcypher.com/v1/doge/main/addrs/" + address + "/full?limit=50"

	var result []Transfer
	fetched := 0 // txs processed, not Transfers: 1 tx = N outputs
	before := 0  // 0 = first page, no cursor yet

	for fetched < limit {
		url := base
		if before > 0 {
			url = fmt.Sprintf("%s&before=%d", base, before)
		}

		var data transactionDOGE
		if err := getJSON(url, &data); err != nil {
			return nil, err
		}

		if len(data.Txs) == 0 {
			break
		}

		minBlock := 0 // oldest block on this page, becomes the next cursor
		for _, tx := range data.Txs {
			if fetched >= limit {
				break
			}

			// coinbase inputs carry no addresses, utxoSender skips them
			var inputs []string
			for _, in := range tx.Inputs {
				if len(in.Addresses) > 0 {
					inputs = append(inputs, in.Addresses[0])
				}
			}
			from := utxoSender(address, inputs)

			// BlockCypher sends the timestamp as RFC3339, not as an int.
			// Unconfirmed txs have an empty field, so ts stays 0.
			var ts int64
			if t, err := time.Parse(time.RFC3339, tx.Confirmed); err == nil {
				ts = t.Unix()
			}

			// one Transfer per addressed output; skips OP_RETURN and non-standard scripts
			for _, out := range tx.Output {
				if len(out.Addresses) == 0 || out.Addresses[0] == "" {
					continue
				}
				result = append(result, Transfer{
					From:      from,
					To:        out.Addresses[0],
					Amount:    float64(out.Value) / 100_000_000,
					Asset:     "DOGE",
					Chain:     "DOGE",
					Timestamp: ts,
					Txhash:    tx.Hash,
				})
			}

			fetched++
			if bh := tx.BlockHeight; bh > 0 && (minBlock == 0 || bh < minBlock) {
				minBlock = bh
			}
		}

		// anti-loop guard: bail out if the cursor is missing or not moving back
		if minBlock == 0 || (before != 0 && minBlock >= before) {
			break
		}
		before = minBlock
	}

	return result, nil
}
