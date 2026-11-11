package src

import (
	"errors"
	"strings"
)

// Base58Check version bytes. Deciding on the decoded version byte instead of the
// first character avoids the old prefix-switch traps: a Solana key starting with
// "2", "m" or "A" is no longer taken for a BTC testnet or a DOGE address.
type base58Kind struct {
	chain   string
	subtype string
	testnet bool
}

var base58Versions = map[byte]base58Kind{
	0x00: {"BTC", "P2PKH", false},
	0x05: {"BTC", "P2SH", false},
	0x6f: {"BTC", "P2PKH", true},
	0xc4: {"BTC", "P2SH", true},
	0x1e: {"DOGE", "P2PKH", false},
	0x16: {"DOGE", "P2SH", false},
	0x41: {"TRON", "", false},
}

func (W *Wallet) Indentification() error {
	wallet := W.Address
	if wallet == "" {
		return errors.New("Wallet is empty")
	}

	// BTC SegWit / Taproot
	low := strings.ToLower(wallet)
	if strings.HasPrefix(low, "bc1") || strings.HasPrefix(low, "tb1") {
		hrp, version, program, ok := segwitDecode(wallet)
		if !ok || (hrp != "bc" && hrp != "tb") {
			return errors.New("Invalid checksum: " + wallet)
		}
		W.Type = "BTC"
		W.Testnet = hrp == "tb"
		switch {
		case version == 0 && len(program) == 20:
			W.Subtype = "BECH32 (P2WPKH)"
		case version == 0:
			W.Subtype = "BECH32 (P2WSH)"
		case version == 1 && len(program) == 32:
			W.Subtype = "BECH32M (Taproot)"
		default:
			W.Subtype = "BECH32M"
		}
		return nil
	}

	// EVM
	if strings.HasPrefix(wallet, "0x") {
		if stringLen(wallet) != 42 || !isHex(wallet[2:]) {
			return errors.New("Wallet is invalid: " + wallet)
		}
		body := wallet[2:]
		switch {
		case checkSumEvm(wallet):
			W.Subtype = "EIP-55"
		// EIP-55: an all-lower or all-upper address carries no checksum at all,
		// and explorers often print them that way, so it is accepted as is
		case body == strings.ToLower(body) || body == strings.ToUpper(body):
			W.Subtype = "no checksum"
		default:
			return errors.New("Invalid checksum: " + wallet)
		}
		W.Type = "ETH"
		return nil
	}

	if !isBase58(wallet) {
		return errors.New("Wallet is invalid: " + wallet)
	}

	// BTC legacy / DOGE / Tron: 1 version byte + 20-byte hash
	if payload, ok := decodeBase58Check(wallet, alphabetBase58); ok && len(payload) == 21 {
		if kind, known := base58Versions[payload[0]]; known {
			W.Type, W.Subtype, W.Testnet = kind.chain, kind.subtype, kind.testnet
			return nil
		}
	}

	// XRP: same scheme, own alphabet, account IDs always start with "r" (version 0x00)
	if strings.HasPrefix(wallet, "r") {
		if payload, ok := decodeBase58Check(wallet, alphabetBase58XRP); ok && len(payload) == 21 && payload[0] == 0 {
			W.Type, W.Subtype = "XRP", ""
			return nil
		}
	}

	// Solana: raw Ed25519 key, no checksum to verify, but it must be exactly 32 bytes
	if raw, ok := decodeBase58(wallet, alphabetBase58); ok && len(raw) == 32 {
		W.Type, W.Subtype = "SOLANA", ""
		return nil
	}

	// decodes to the size of a checksummed address: almost surely a typo
	if raw, _ := decodeBase58(wallet, alphabetBase58); len(raw) == 25 {
		return errors.New("Invalid checksum: " + wallet)
	}
	return errors.New("Wallet is invalid: " + wallet)
}
