//go:build integration

// Live tests against the real explorers. They need network access and are subject
// to rate limits, so they are kept out of the default run:
//
//	go test -tags integration ./src -v
package src

import "testing"

func TestConnectorsLive(t *testing.T) {
	cases := []struct {
		addr string
	}{
		{"1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa"},
		{"mipcBbFg9gMiCh81Kj8tqqdgoZub1ZJRfn"},
		{"0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045"},
		{"DH5yaieqoZN36fDVciNyRueRGvGLR3mr7L"},
		{"TV6MuMXfmLbBqPZvBHdwFsDnQeVfnmiuSi"},
		{"rEb8TK3gBgk5auZkwc6sHnwrGVJH8DuaLh"},
		{"9WzDXwBbmkg8ZTbNMqUxvQRAyrZzDsGYdLVL9zYtAWWM"},
	}
	for _, c := range cases {
		w := Wallet{Address: c.addr}
		if err := w.Indentification(); err != nil {
			t.Errorf("%s: %v", c.addr, err)
			continue
		}
		conn, err := ConnectorFor(w)
		if err != nil {
			t.Errorf("%s: %v", c.addr, err)
			continue
		}
		b, err := conn.GetBalance(c.addr)
		if err != nil {
			t.Errorf("%s %s balance: %v", w.Type, c.addr, err)
			continue
		}
		txs, err := conn.GetTransactions(c.addr, 25)
		if err != nil {
			t.Errorf("%s %s transactions: %v", w.Type, c.addr, err)
			continue
		}
		assets := map[string]int{}
		touches := 0
		for _, tx := range txs {
			assets[tx.Asset]++
			if SameAddress(w.Type, tx.From, c.addr) || SameAddress(w.Type, tx.To, c.addr) {
				touches++
			}
		}
		if len(txs) > 0 && touches == 0 {
			t.Errorf("%s: no transfer involves the address itself", c.addr)
		}
		t.Logf("%-6s testnet=%-5v balance=%.4f txcount=%d transfers=%d touching=%d assets=%v",
			w.Type, w.Testnet, b.Balance, b.TxCount, len(txs), touches, assets)
	}
}

func TestErrorsLive(t *testing.T) {
	// valid format, rejected by the explorer: must be a readable error, not a panic
	if _, err := (ETHConnector{}).GetBalance("0xnotanaddress"); err == nil {
		t.Error("ETH: bad address accepted")
	} else {
		t.Logf("ETH bad address -> %v", err)
	}
	// valid checksum, never funded
	raw := []byte{0x00, 9, 8, 7, 6, 5, 4, 3, 2, 1, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	xrp := encodeBase58Check(raw, alphabetBase58XRP)
	if _, err := (XRPConnector{}).GetBalance(xrp); err == nil {
		t.Error("XRP: unfunded account gave no error")
	} else {
		t.Logf("XRP unfunded %s -> %v", xrp, err)
	}
}
