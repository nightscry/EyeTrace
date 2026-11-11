package src



type addressResponseBTC struct{
	ChainStats struct{
		FundedSum int64 `json:"funded_txo_sum"`
		SpentSum int64 `json:"spent_txo_sum"`
		TxCount int `json:"tx_count"`
	}`json:"chain_stats"`
}

type transactionsBTC struct{
	TxHash string `json:"txid"`
	Vin []struct{
		IsCoinBase bool `json:"is_coinbase"`
		Prevout struct{
			ScriptPubkeyAddress string `json:"scriptpubkey_address"`
		}`json:"prevout"`
	} `json:"vin"`
	Vout []struct{
		ScriptPubkeyAddres string `json:"scriptpubkey_address"`
		Value float64 `json:"value"`
	} `json:"vout"`
	Status struct{
		BlockTime int64 `json:"block_time"`
	}`json:"status"`
}






// Blockstream serves testnet under its own prefix, same API shape.
func (B BTCconnector) apiBase() string {
	if B.Testnet {
		return "https://blockstream.info/testnet/api"
	}
	return "https://blockstream.info/api"
}

func (B BTCconnector)GetBalance(address string)(BalanceTxCount,error){
	url := B.apiBase() + "/address/" + address

	var data addressResponseBTC
	if err := getJSON(url, &data); err != nil{
		return BalanceTxCount{}, err
	}

	satoshi := data.ChainStats.FundedSum - data.ChainStats.SpentSum



	result := BalanceTxCount{
		Balance: float64(satoshi) / 100_000_000,
		TxCount: data.ChainStats.TxCount,
	}
	return result, nil
}

// Blockstream has no "limit" parameter: /txs always returns the 25 newest txs.
// Going further means paging through /txs/chain/{last_seen_txid}, 25 at a time.
func (B BTCconnector)GetTransactions(address string, limit int)([]Transfer,error){
	if limit <= 0 {
		limit = 25
	}

	base := B.apiBase() + "/address/" + address + "/txs"

	var result []Transfer
	fetched := 0   // txs processed, not Transfers: 1 tx = N outputs
	lastSeen := "" // "" = first page

	for fetched < limit {
		url := base
		if lastSeen != "" {
			url = base + "/chain/" + lastSeen
		}

		var page []transactionsBTC
		if err := getJSON(url, &page); err != nil {
			return nil, err
		}

		if len(page) == 0 {
			break
		}

		for _, d := range page {
			if fetched >= limit {
				break
			}

			var inputs []string
			for _, vin := range d.Vin {
				if !vin.IsCoinBase {
					inputs = append(inputs, vin.Prevout.ScriptPubkeyAddress)
				}
			}
			from := utxoSender(address, inputs)

			// one Transfer per addressed output; skips OP_RETURN and non-standard scripts
			for _, vout := range d.Vout {
				if vout.ScriptPubkeyAddres == "" {
					continue
				}
				result = append(result, Transfer{
					From:      from,
					To:        vout.ScriptPubkeyAddres,
					Amount:    vout.Value / 100_000_000,
					Asset:     "BTC",
					Chain:     "BTC",
					Timestamp: d.Status.BlockTime,
					Txhash:    d.TxHash,
				})
			}

			fetched++
			lastSeen = d.TxHash
		}

		// Blockstream pages in blocks of 25, so a short page is the end of history
		if len(page) < 25 {
			break
		}
	}

	return result, nil
}
