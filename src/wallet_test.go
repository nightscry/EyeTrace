package src

import "testing"

func TestIdentification(t *testing.T) {
	cases := []struct {
		addr    string
		chain   string // "" = must be rejected
		testnet bool
	}{
		// BTC mainnet
		{"1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa", "BTC", false},
		{"17VZNX1SN5NtKa8UQFxwQbFeFc3iqRYhem", "BTC", false},
		{"3J98t1WpEZ73CNmQviecrnyiWrnqRhWNLy", "BTC", false},
		{"342ftSRCvFHfCeFFBuz4xwbeqnDw6BGUey", "BTC", false},
		{"bc1qw508d6qejxtdg4y5r3zarvary0c5xw7kv8f3t4", "BTC", false},
		{"BC1QW508D6QEJXTDG4Y5R3ZARVARY0C5XW7KV8F3T4", "BTC", false}, // BIP173: all-uppercase is valid
		{"bc1qrp33g0q5c5txsp9arysrx4k6zdkfs4nce4xj0gdcccefvpysxf3qccfmv3", "BTC", false},
		{"bc1p5cyxnuxmeuwuvkwfem96lqzszd02n6xdcjrs20cac6yqjjwudpxqkedrcr", "BTC", false},
		{"bc1p0xlxvlhemja6c4dqv22uapctqupfhlxm9h8z3k2e72q4k9hcz7vqzk5jj0", "BTC", false}, // BIP350 vector
		// BTC testnet
		{"mipcBbFg9gMiCh81Kj8tqqdgoZub1ZJRfn", "BTC", true},
		{"n2ZNV88uQbede7C5M5jzi6SyG4GVuPpng6", "BTC", true},
		{"2MzQwSSnBHWHqSAqtTVQ6v47XtaisrJa1Vc", "BTC", true},
		{"tb1qw508d6qejxtdg4y5r3zarvary0c5xw7kxpjzsx", "BTC", true},
		{"tb1pqqqqp399et2xygdj5xreqhjjvcmzhxw4aywxecjdzew6hylgvsesf3hn0c", "BTC", true},
		// EVM (EIP-55 spec vectors)
		{"0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed", "ETH", false},
		{"0xfB6916095ca1df60bB79Ce92cE3Ea74c37c5d359", "ETH", false},
		{"0xdbF03B407c01E7cD3CBea99509d93f8DDDC8C6FB", "ETH", false},
		{"0xD1220A0cf47c7B9Be7A2E6BA89F429762e7b9aDb", "ETH", false},
		{"0xde0B295669a9FD93d5F28D9Ec85E40f4cb697BAe", "ETH", false},
		{"0xd8da6bf26964af9d7eed9e03e53415d37aa96045", "ETH", false}, // lowercase: no checksum, accepted
		// DOGE
		{"DH5yaieqoZN36fDVciNyRueRGvGLR3mr7L", "DOGE", false},
		{"DMQ7touh2zomesS59FVFKEcTKPkG8YoEYc", "DOGE", false},
		// Tron
		{"TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t", "TRON", false},
		// XRP
		{"rEb8TK3gBgk5auZkwc6sHnwrGVJH8DuaLh", "XRP", false},
		{"rLHzPsX6oXkzU2qL12kHCH8G8cnZv1rBJh", "XRP", false},
		{"rrrrrrrrrrrrrrrrrrrrBZbvji", "XRP", false},
		// Solana
		{"9WzDXwBbmkg8ZTbNMqUxvQRAyrZzDsGYdLVL9zYtAWWM", "SOLANA", false},
		{"So11111111111111111111111111111111111111112", "SOLANA", false},

		// rejected: one character changed in a valid address
		{"1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNb", "", false},
		{"3J98t1WpEZ73CNmQviecrnyiWrnqRhWNLz", "", false},
		{"DH5yaieqoZN36fDVciNyRueRGvGLR3mr7K", "", false},
		{"TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6u", "", false},
		{"bc1qw508d6qejxtdg4y5r3zarvary0c5xw7kv8f3t5", "", false},
		{"0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAeD", "", false},
		// rejected: once listed as valid in addressTest.tt, but their checksums do not
		// verify (cross-checked with an independent implementation)
		{"0xde0B295669a9FD93d5F28D9Ec85E40f4cB697BAe", "", false},
		{"DBKr5akdkAsBrQr4x1qNQ5DsZfLbEMFB5c", "", false},
		{"A2fCS44sYY9dpFDLFdcbWkVUUj2iBqx8gY", "", false},
		{"9vT8kFAghF1Jt73GxSGrhCPYWSEDeYqTuh", "", false},
		// rejected: BIP173/BIP350 rules beyond the checksum
		{"bc1qW508d6qejxtdg4y5r3zarvary0c5xw7kv8f3t4", "", false}, // mixed case
		{"bc1qw508d6qejxtdg4y5r3zarvary0c5xw7kemeawh", "", false}, // v0 with a bech32m checksum
		// rejected: garbage
		{"non-un-indirizzo", "", false},
		{"", "", false},
		{"0x123", "", false},
		// a 33-char base58 string decodes to 24 bytes: too short for an Ed25519 key
		{"DYw8jCTfwHNRJhhmFcbXvVDTqWMEVFBX6", "", false},
	}

	for _, c := range cases {
		w := Wallet{Address: c.addr}
		err := w.Indentification()
		if c.chain == "" {
			if err == nil {
				t.Errorf("%q: accepted as %s %s, want rejection", c.addr, w.Type, w.Subtype)
			}
			continue
		}
		if err != nil {
			t.Errorf("%q: %v, want %s", c.addr, err, c.chain)
			continue
		}
		if w.Type != c.chain || w.Testnet != c.testnet {
			t.Errorf("%q: got %s testnet=%v, want %s testnet=%v", c.addr, w.Type, w.Testnet, c.chain, c.testnet)
		}
	}
}

// Every Base58Check version byte must map to its chain, whatever the first letter.
func TestBase58Versions(t *testing.T) {
	for version, kind := range base58Versions {
		payload := append([]byte{version}, make([]byte, 20)...)
		payload[20] = 7
		w := Wallet{Address: encodeBase58Check(payload, alphabetBase58)}
		if err := w.Indentification(); err != nil || w.Type != kind.chain || w.Subtype != kind.subtype || w.Testnet != kind.testnet {
			t.Errorf("version 0x%02x (%s): got %s/%s testnet=%v err=%v", version, w.Address, w.Type, w.Subtype, w.Testnet, err)
		}
	}
}

func TestBase58CheckRoundTrip(t *testing.T) {
	// USDT contract on Tron, hex as TronGrid returns it
	if got := tronHexToBase58("41a614f803b6fd780986a42c78ec9c7f77e6ded13c"); got != "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t" {
		t.Fatalf("tronHexToBase58 = %s", got)
	}
	if got := tronHexToBase58("zz"); got != "" {
		t.Fatalf("garbage hex should give empty, got %s", got)
	}
	// leading zero bytes must survive the round trip as leading '1's
	payload := []byte{0, 0, 1, 2, 3}
	s := encodeBase58Check(payload, alphabetBase58)
	back, ok := decodeBase58Check(s, alphabetBase58)
	if !ok || string(back) != string(payload) {
		t.Fatalf("round trip: %q -> %v %v", s, back, ok)
	}
}

func TestUtxoSender(t *testing.T) {
	if got := utxoSender("T", []string{"A", "T"}); got != "T" {
		t.Errorf("target as 2nd input: got %q", got)
	}
	if got := utxoSender("T", []string{"", "B", "C"}); got != "B" {
		t.Errorf("first addressed input: got %q", got)
	}
	if got := utxoSender("T", nil); got != "" {
		t.Errorf("coinbase: got %q", got)
	}
}
