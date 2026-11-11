package src

import "strings"

// Entity categories. Only mixers raise the risk flag: an exchange is not suspicious
// by itself, it is where funds usually enter or leave the system.
const (
	CatExchange = "exchange"
	CatMixer    = "mixer"
	CatContract = "contract"
	CatToken    = "token"
	CatBurn     = "burn"
	CatOther    = "other"
)

type Entity struct {
	Name     string
	Category string
}

func (e Entity) Risky() bool { return e.Category == CatMixer }

// Built-in labels. Contract names were checked on-chain (verified source on
// Blockscout); exchange wallets are community labels, widely published but not
// provable on-chain. Keys are chain + ":" + address, EVM lowercased.
var knownEntities = map[string]Entity{
	// ETH: Tornado Cash pools and router (OFAC-sanctioned in 2022, delisted in 2025)
	"ETH:0x12d66f87a04a9e220743712ce6d9bb1b5616b8fc": {"Tornado Cash 0.1 ETH", CatMixer},
	"ETH:0x47ce0c6ed5b0ce3d3a51fdb1c52dc66a7c3c2936": {"Tornado Cash 1 ETH", CatMixer},
	"ETH:0x910cbd523d972eb0a6f4cae4618ad62622b39dbf": {"Tornado Cash 10 ETH", CatMixer},
	"ETH:0xa160cdab225685da1d56aa342ad8841c3b53f291": {"Tornado Cash 100 ETH", CatMixer},
	"ETH:0xd90e2f925da726b50c4ed8d0fb90ad053324f31b": {"Tornado Cash Router", CatMixer},
	// ETH: exchanges
	"ETH:0xf977814e90da44bfa03b6295a0616a897441acec": {"Binance 8", CatExchange},
	"ETH:0x28c6c06298d514db089934071355e5743bf21d60": {"Binance 14", CatExchange},
	"ETH:0x21a31ee1afc51d94c2efccaa2092ad1028285549": {"Binance 15", CatExchange},
	"ETH:0xdfd5293d8e347dfe59e90efd55b2956a1343963d": {"Binance 16", CatExchange},
	// ETH: protocol contracts and tokens
	"ETH:0x00000000219ab540356cbb839cbe05303d7705fa": {"Beacon Deposit Contract", CatContract},
	"ETH:0xdac17f958d2ee523a2206206994597c13d831ec7": {"USDT (Tether)", CatToken},
	"ETH:0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48": {"USDC", CatToken},
	"ETH:0x6b175474e89094c44da98b954eedeac495271d0f": {"DAI", CatToken},
	"ETH:0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2": {"WETH", CatToken},
	"ETH:0x2260fac5e5542a773aa44fbcfedf7c193bc2c599": {"WBTC", CatToken},
	"ETH:0x0000000000000000000000000000000000000000": {"Null address", CatBurn},
	"ETH:0x000000000000000000000000000000000000dead": {"Burn address", CatBurn},
	// TRON
	"TRON:TV6MuMXfmLbBqPZvBHdwFsDnQeVfnmiuSi": {"Binance", CatExchange},
	"TRON:TLsV52sRDL79HXGGm9yzwKibb6BeruhUzy": {"Tron blackhole (fee burn)", CatBurn},
	"TRON:TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t": {"USDT (Tether) contract", CatToken},
	// BTC
	"BTC:1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa":                             {"Genesis address (Satoshi)", CatOther},
	"BTC:34xp4vRoCGJym3xR7yCVPFHoCNxv4Twseo":                             {"Binance cold wallet", CatExchange},
	"BTC:bc1qgdjqv0av3q56jvd82tkdjpy7gdp9ut8tlqmgrpmv24sq90ecnvqqjwvw97": {"Bitfinex cold wallet", CatExchange},
	// XRP
	"XRP:rrrrrrrrrrrrrrrrrrrrBZbvji":         {"XRPL black hole (ACCOUNT_ONE)", CatBurn},
	"XRP:rMxCKbEDwqr76QuheSUMdEGf4B9xJ8m5De": {"RLUSD issuer (Ripple)", CatToken},
}

// EVM addresses compare case-insensitively (explorers return them lowercase,
// users paste them checksummed); every other chain is case-sensitive.
func entityKey(chain, address string) string {
	if chain == "ETH" {
		address = strings.ToLower(address)
	}
	return chain + ":" + address
}

func LookupEntity(chain, address string) (Entity, bool) {
	e, ok := knownEntities[entityKey(chain, address)]
	return e, ok
}
