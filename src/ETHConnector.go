package src

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"net/url"
	"os"
	"strconv"
	"strings"
)

const blockscoutAPI = "https://eth.blockscout.com/api"

// Keyless by default. Blockscout allows only a handful of keyless requests per window,
// and every expansion costs three, so heavy users can set a free key from
// blockscout.com in EYETRACE_BLOCKSCOUT_KEY.
func blockscoutKey() string {
	if k := os.Getenv("EYETRACE_BLOCKSCOUT_KEY"); k != "" {
		return "&apikey=" + url.QueryEscape(k)
	}
	return ""
}

// errors Blockscout reports inside the JSON envelope, as opposed to HTTP failures
type blockscoutError struct{ msg string }

func (e blockscoutError) Error() string { return "Blockscout: " + e.msg }

// Etherscan-style envelope: errors come back as HTTP 200 with status "0",
// and result is then null or an error string instead of the expected type.
type ethEnvelope struct{
	Status string `json:"status"`
	Message string `json:"message"`
	Result json.RawMessage `json:"result"`
}

// one row shape covers txlist, txlistinternal and tokentx: the fields a list
// does not have simply stay empty
type ethHistory []struct{
	From string `json:"from"`
	To string `json:"to"`
	Value string `json:"value"`
	TimeStamp string `json:"timestamp"`
	Hash string `json:"hash"`
	IsError string `json:"isError"`
	ContractAddress string `json:"contractAddress"`
	TokenSymbol string `json:"tokenSymbol"`
	TokenDecimal string `json:"tokenDecimal"`
}

// v2 address view: balance plus what Blockscout knows about the address itself
type ethAddressV2 struct{
	CoinBalance string `json:"coin_balance"`
	Name string `json:"name"` // verified contract name, e.g. "TornadoCash_eth"
	IsContract bool `json:"is_contract"`
}

type ethCounters struct{
	TransactionsCount string `json:"transactions_count"`
}

// Contracts of the main ERC-20s, lowercase as the API returns them.
// Anyone can deploy a token called "USDT", so the symbol alone proves nothing.
var ethKnownTokens = map[string]bool{
	"0xdac17f958d2ee523a2206206994597c13d831ec7": true, // USDT
	"0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48": true, // USDC
	"0x6b175474e89094c44da98b954eedeac495271d0f": true, // DAI
	"0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2": true, // WETH
	"0x2260fac5e5542a773aa44fbcfedf7c193bc2c599": true, // WBTC
}

func ethGet(url string, out any) error {
	var env ethEnvelope
	if err := getJSON(url, &env); err != nil {
		return err
	}
	// "No transactions found" (and "No token transfers found") are status 0 too,
	// but they are an empty history, not an error
	if env.Status != "1" && !strings.HasPrefix(env.Message, "No ") {
		return blockscoutError{env.Message}
	}
	if env.Status != "1" {
		return nil
	}
	if err := json.Unmarshal(env.Result, out); err != nil {
		return fmt.Errorf("Blockscout: unexpected result: %w", err)
	}
	return nil
}

// Blockscout returns wei as a decimal string, so it goes through big, never float
func ConversionBigToFloat(value string,base float64)(float64){


	wei := new(big.Int)
	wei.SetString(value,10)

	ethF := new(big.Float).SetInt(wei)
	ethF.Quo(ethF,big.NewFloat(base))

	result, _ := ethF.Float64()
	return result
}


func (Eth ETHConnector)GetBalance(address string)(BalanceTxCount,error){
	result := BalanceTxCount{}

	// One v2 call gives the balance and the on-chain identity (contract or not, verified
	// name): that is what lets expanded wallets be recognised beyond the built-in labels.
	// The v1 balance is the fallback, it also turns a bad address into a clear error.
	var info ethAddressV2
	if err := getJSON("https://eth.blockscout.com/api/v2/addresses/"+address, &info); err == nil && info.CoinBalance != "" {
		result.Balance = ConversionBigToFloat(info.CoinBalance, 1e18)
		result.Name, result.IsContract = info.Name, info.IsContract
	} else {
		var wei string
		if err := ethGet(blockscoutAPI+"?module=account&action=balance&address="+address+blockscoutKey(), &wei); err != nil { return BalanceTxCount{}, err }
		result.Balance = ConversionBigToFloat(wei, 1e18)
	}

	// the count lives only in the v2 API; it is a nice-to-have, so a failure here
	// must not cost the user the balance
	var counters ethCounters
	if err := getJSON("https://eth.blockscout.com/api/v2/addresses/"+address+"/counters", &counters); err == nil {
		result.TxCount, _ = strconv.Atoi(counters.TransactionsCount)
	}

	return result, nil
}

// Three lists, up to limit each: plain ETH transfers, ETH moved by contracts
// (internal txs: withdrawals, mixers, bridges all pay out this way) and ERC-20s.
// Here offset IS the page size, so one call per list is enough (up to 10k).
func (Eth ETHConnector)GetTransactions(address string, limit int)([]Transfer,error){
	if limit <= 0 {
		limit = 25
	}
	if limit > 10000 {
		limit = 10000 // page * offset is capped at 10k by the API
	}

	var result []Transfer
	for _, action := range []string{"txlist", "txlistinternal", "tokentx"} {
		u := fmt.Sprintf("%s?module=account&action=%s&address=%s&sort=desc&page=1&offset=%d%s", blockscoutAPI, action, address, limit, blockscoutKey())

		var data ethHistory
		if err := ethGet(u, &data); err != nil {
			// internal txs and token transfers are extras: Blockscout sometimes refuses
			// them while it is still indexing a block range, and that must not cost
			// the user the plain history. Network errors and 429s are not skipped:
			// silently dropping a list would draw an incomplete graph.
			var be blockscoutError
			if action != "txlist" && errors.As(err, &be) {
				continue
			}
			return nil, err
		}

		for _,object := range data{
			// a reverted tx moved no value
			if object.IsError == "1" {
				continue
			}

			asset, base := "ETH", 1e18
			if action == "tokentx" {
				decimals, _ := strconv.Atoi(object.TokenDecimal)
				asset, base = object.TokenSymbol, math.Pow10(decimals)
				if !ethKnownTokens[strings.ToLower(object.ContractAddress)] {
					asset += UnverifiedMark
				}
			}

			amount := ConversionBigToFloat(object.Value, base)
			// zero-value calls are contract interactions, not transfers; zero-value
			// token transfers are the classic address-poisoning trick
			if amount == 0 {
				continue
			}

			time, _ := strconv.Atoi(object.TimeStamp)
			result = append(result, Transfer{
				From: object.From,
				To: object.To,
				Amount: amount,
				Timestamp: int64(time),
				Asset: asset,
				Chain: "ETH",
				Txhash: object.Hash,
			})
		}
	}

	return result, nil
}
