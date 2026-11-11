<div align="center">

<img src="gui/assets/logo.png" alt="EyeTrace" width="280"/>

# EyeTrace

**See every transaction. Trace every wallet.**

Multi-chain blockchain intelligence and wallet transaction tracer, with an interactive force-directed graph.

![Go](https://img.shields.io/badge/Go-1.25-00ADD8?logo=go&logoColor=white)
![Fyne](https://img.shields.io/badge/GUI-Fyne%20v2-1a1a2e)
![Version](https://img.shields.io/badge/version-1.0.0-a855f7)
![Keyless](https://img.shields.io/badge/API-keyless-8b5cf6)
![Platform](https://img.shields.io/badge/platform-macOS%20%7C%20Linux%20%7C%20Windows-040309)

<sub>a NightScry project</sub>

</div>

---

## What it is

EyeTrace takes a wallet address, detects its blockchain, fetches balance and transactions, and draws an interactive graph of its counterparties. You can expand any counterparty to follow the money hop by hop, spot known exchanges and mixers, and save the whole investigation to reopen it later or export it as CSV.

Built for on-chain investigative analysis: following the flow of money between wallets, exchanges, and services.

---

## Features

- **Automatic chain detection** from the address alone, with full cryptographic validation (not just pattern matching):
  - Bitcoin Base58Check (P2PKH, P2SH) and Bech32 / Bech32m (SegWit, Taproot), mainnet and testnet, with the BIP350 witness-version rules
  - Ethereum EIP-55 (Keccak-256); all-lowercase addresses, which carry no checksum, are accepted as well
  - Dogecoin and Tron Base58Check, told apart by the decoded version byte rather than the first letter
  - XRP Base58Check (XRPL alphabet)
  - Solana: must decode to exactly 32 bytes (an Ed25519 key has no checksum)
- **Six keyless connectors** for BTC, ETH, DOGE, TRON, XRP and SOL, with timeouts, rate-limit handling and readable errors
- **Tokens as well as native coins**: ERC-20, TRC-20, SPL and XRPL issued currencies. Tokens are identified by contract, mint or issuer, never by symbol, so a fake "USDT" is flagged `USDT?` and kept apart from the real one
- **Multi-hop graph**: double-click a counterparty (or press *Expand*) to fetch its history and grow the graph past one hop. Each expanded wallet becomes its own star, pushed away from the target, and the view re-frames itself to show it
- **Every expanded wallet is identified like the target**: type, balance, transaction count, its on-chain identity (verified contract names on ETH) and an activity profile that flags exchange- or service-like volume as a heuristic
- **Direction and volume**: arrows show who paid whom, and colours show sent / received / both from the point of view of each analysed wallet. Edge width, edge opacity and node size all follow volume on a log scale
- **Zoom and pan**: scroll to zoom, drag the background to pan, double-click empty space to reset the view
- **Entity labels and risk**: a built-in list of exchanges, mixers (Tornado Cash), token contracts and burn addresses; the panel reports any mixer reachable from the target and its distance in hops
- **Asset selector**: one asset per graph, since summing TRX with USDT means nothing
- **Investigations**: *Export* saves `.json` (the full case, reopenable offline with *Open*) or `.csv` (a flat table of transfers with labels)
- **Native UI** with the NightScry theme (Fyne)

---

## Supported chains

| Chain | Detection | Data source (keyless) | Assets |
|-------|-----------|-----------------------|--------|
| Bitcoin (Legacy/SegWit/Taproot, mainnet + testnet) | Base58Check, Bech32/m | Blockstream | BTC |
| Ethereum | EIP-55 | Blockscout | ETH, internal txs, ERC-20 |
| Dogecoin | Base58Check | BlockCypher | DOGE |
| Tron | Base58Check | TronGrid | TRX, TRC-20 |
| XRP Ledger | Base58Check (XRPL alphabet) | XRPL public node | XRP, issued currencies |
| Solana | 32-byte key | Solana public RPC | SOL, SPL tokens |

**What gets counted:**
- Failed and reverted transactions are dropped, and so are zero-value transfers (the usual address-poisoning trick).
- On XRPL the amount is `delivered_amount`, never `Amount`, which a partial payment can inflate.
- On Solana, token movements are read from per-owner balance changes, because transfer instructions name token accounts rather than wallets.

---

## Getting Started

```bash
# clone
git clone https://github.com/nightscry/EyeTrace.git && cd EyeTrace

# dependencies
go mod download

# run
go run .

# or open straight on an address
go run . 0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045
```

No API keys needed. Keyless explorers do rate-limit, though: Blockscout in particular allows only a few requests per window, and each ETH expansion costs three. If you expand a lot, put a free Blockscout key in `EYETRACE_BLOCKSCOUT_KEY`. When a limit is hit, EyeTrace tells you how long it lasts instead of retrying blindly.

> Requirements: Go 1.25+, a C compiler (for Fyne/OpenGL — on macOS, the Xcode Command Line Tools).

Try any address, for example:

```
1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa                  (BTC — genesis block)
0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045          (ETH — vitalik.eth)
TV6MuMXfmLbBqPZvBHdwFsDnQeVfnmiuSi                  (TRON — Binance hot wallet)
rEb8TK3gBgk5auZkwc6sHnwrGVJH8DuaLh                  (XRP)
9WzDXwBbmkg8ZTbNMqUxvQRAyrZzDsGYdLVL9zYtAWWM        (SOL)
```

**In the graph:** click a node for its details (its neighbourhood stays lit, the rest fades), double-click to expand it, hover to see an address, scroll to zoom, drag a node to move it, drag the background to pan. Only names and each wallet's top counterparties are labelled by default.

### Tests

```bash
go test ./...                              # offline: detection, checksums, parsers, graph logic
go test -tags integration ./src -v         # live, against the real explorers
```

---

## Architecture

```
Address -> Detect chain -> Connector -> []Transfer -> Investigation -> Graph builder -> GUI
           (checksum +     (keyless,    (normalized   (expanded         (per asset,     (Fyne)
            version byte)   retries)     model)        histories)        multi-hop)
```

```
eyetracker/
├── eyetracker.go            # entrypoint -> gui.RunGUI([address])
├── FyneApp.toml             # app metadata for `fyne package`
├── src/                     # core (package src), no GUI dependency
│   ├── wallet.go            # Wallet + Identification (detection)
│   ├── checksum.go          # Base58Check, Bech32/m segwit decode, EIP-55
│   ├── validate.go          # alphabets, validation, UTXO sender rule
│   ├── model.go             # Transfer, Wallet, Connector interface, ConnectorFor
│   ├── httpclient.go        # shared client: timeout, 429 handling, errors
│   ├── labels.go            # known entities (exchange, mixer, ...)
│   ├── investigation.go     # case model, dedup, JSON save/load, CSV export
│   ├── BTCconnector.go      # Blockstream (UTXO model, mainnet + testnet)
│   ├── ETHConnector.go      # Blockscout (txs + internal + ERC-20)
│   ├── DOGEConnector.go     # BlockCypher (UTXO model)
│   ├── TRONConnector.go     # TronGrid (TRX + TRC-20)
│   ├── XRPConnector.go      # XRPL JSON-RPC
│   └── SolanaConnector.go   # Solana JSON-RPC (batched)
└── gui/                     # interface (package gui)
    ├── app.go               # app metadata + theme + splash -> dashboard
    ├── theme.go             # NightScry theme (palette + embedded logo)
    ├── splash.go            # loading screen
    ├── dashboard.go         # main window: search, depth, asset, open/export, expansion
    ├── graph.go             # multi-hop force-directed graph, zoom/pan, arrows
    └── panel.go             # selected node details, actions, risk
```

Core design: a shared `Connector` interface and a chain-agnostic `Transfer`, so adding a chain means one connector file plus one line in `ConnectorFor`, and neither the graph nor the GUI changes.

---

## Roadmap

- [x] Multi-chain detector with checksum and version-byte validation
- [x] Keyless connectors for BTC, ETH, DOGE, TRON, XRP and SOL behind a common interface
- [x] Tokens (ERC-20, TRC-20, SPL, XRPL IOUs), with fake tokens flagged
- [x] Multi-hop expansion (counterparties of counterparties)
- [x] Zoom / pan / directional arrows on the graph
- [x] Entity labels (exchange, mixer, contract, burn) + mixer exposure
- [x] Save / reopen investigations, CSV export
- [ ] Persistent cache (SQLite) to spare the explorers' rate limits
- [ ] Larger, updatable label sets (import from a file) and a proper risk score
- [ ] AI-assisted explanations (plain-language summary of an investigation)
- [ ] More EVM chains (BSC, Polygon, Arbitrum: same Blockscout API)

---

## Stack

Go, Fyne v2 (native GUI), golang.org/x/crypto (Keccak-256), math/big (wei/satoshi precision), keyless explorers and nodes (Blockstream, Blockscout, BlockCypher, TronGrid, XRPL, Solana RPC).

---

## License

MIT — see [LICENSE](LICENSE).

---

## Disclaimer

EyeTrace analyzes public on-chain data for research, education, and authorized investigative analysis. Use it responsibly and in compliance with applicable laws.

---

<div align="center">
<sub>a NightScry project</sub>
</div>
