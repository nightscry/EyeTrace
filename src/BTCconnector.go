package src

import (
	"encoding/json"
	"net/http"
)


//struttura della response per leggere i dati get balance di BTC
type addressResonse struct{
	ChainStats struct{
		FundedSum int64 `json:"funded_txo_sum"`
		SpentSum int64 `json:"spent_txo_sum"`
		TxCount int `json:"tx_count"`
	}`json:"chain_stats"`
}

//struttura della risposte per leggere i dati get transaction di BTC
type transactions struct{
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





//funzione get balance BTC
func (B BTCconnector)GetBalance(address string)(float64,error){
	url := "https://blockstream.info/api/address/"+address

	resp, err := http.Get(url)
	if(err!=nil){
		return 0, err
	}
	defer resp.Body.Close()

	var data addressResonse
	if err:= json.NewDecoder(resp.Body).Decode(&data); err != nil{
		return 0, err
	}

	satoshi := data.ChainStats.FundedSum - data.ChainStats.SpentSum
	return float64(satoshi) / 100_000_000, nil

}

//funzione get transaction per BTC
func (B BTCconnector)GetTransactions(address string)([]Transfer,error){
	url := "https://blockstream.info/api/address/"+address+"/txs"
	
	resp, err := http.Get(url)
	if(err!= nil){
		return nil, err
	}

	defer resp.Body.Close()

	var data []transactions
	if err := json.NewDecoder(resp.Body).Decode(&data); err !=nil{
		return nil, err
	}

	var result []Transfer

	for _, d := range data {
		// trova il mittente: il primo vin con un indirizzo valido.
		// Salta i coinbase (non hanno un vero input) e gli input senza indirizzo.
		from := ""
		for _, vin := range d.Vin {
			if vin.IsCoinBase {
				continue
			}
			if vin.Prevout.ScriptPubkeyAddress != "" {
				from = vin.Prevout.ScriptPubkeyAddress
				break
			}
		}

		// crea un Transfer per ogni output con un indirizzo valido.
		// Salta gli output senza indirizzo (OP_RETURN, script non standard).
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
	}

	return result, nil
}



