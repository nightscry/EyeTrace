package main

import (
	"eyetracker/src"
	"fmt"
)

func main(){
	addresses := []string{
      // --- MAINNET ---
      "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",                              // P2PKH  (prefisso 1)
      "17VZNX1SN5NtKa8UQFxwQbFeFc3iqRYhem",                              // P2PKH  (prefisso 1)
      "3J98t1WpEZ73CNmQviecrnyiWrnqRhWNLy",                              // P2SH   (prefisso 3)
      "342ftSRCvFHfCeFFBuz4xwbeqnDw6BGUey",                              // P2SH   (prefisso 3)
      "bc1qw508d6qejxtdg4y5r3zarvary0c5xw7kv8f3t4",                     // Bech32 P2WPKH (bc1q, corto)
      "bc1qrp33g0q5c5txsp9arysrx4k6zdkfs4nce4xj0gdcccefvpysxf3qccfmv3", // Bech32 P2WSH  (bc1q, lungo)
      "bc1p5cyxnuxmeuwuvkwfem96lqzszd02n6xdcjrs20cac6yqjjwudpxqkedrcr", // Bech32m P2TR Taproot (bc1p)

      // --- TESTNET ---
      "mipcBbFg9gMiCh81Kj8tqqdgoZub1ZJRfn",                             // P2PKH testnet (prefisso m)
      "n2ZNV88uQbede7C5M5jzi6SyG4GVuPpng6",                             // P2PKH testnet (prefisso n)
      "2MzQwSSnBHWHqSAqtTVQ6v47XtaisrJa1Vc",                            // P2SH  testnet (prefisso 2)
      "tb1qw508d6qejxtdg4y5r3zarvary0c5xw7kxpjzsx",                     // Bech32 testnet (tb1q)
      "tb1pqqqqp399et2xygdj5xreqhjjvcmzhxw4aywxecjdzew6hylgvsesf3hn0c", // Bech32m testnet Taproot (tb1p)

      // --- NEGATIVI (devono dare UNKNOWN / non-BTC) ---
      "0xde0B295669a9FD93d5F28D9Ec85E40f4cB697BAe",                     // EVM, non BTC
      "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t",                             // Tron, non BTC
      "non-un-indirizzo",                                              // spazzatura
      "",                                                              // stringa vuota
	  "DH5yaieqoZN36fDVciNyRueRGvGLR3mr7L",   // DOGE P2PKH (D)
	"DBKr5akdkAsBrQr4x1qNQ5DsZfLbEMFB5c",   // DOGE P2PKH (D)
	"A2fCS44sYY9dpFDLFdcbWkVUUj2iBqx8gY",   // DOGE P2SH  (A)
	"9vT8kFAghF1Jt73GxSGrhCPYWSEDeYqTuh",   // DOGE P2SH  (9)
	"rEb8TK3gBgk5auZkwc6sHnwrGVJH8DuaLh",   // XRP
	"rLHzPsX6oXkzU2qL12kHCH8G8cnZv1rBJh",   // XRP	
	"DYw8jCTfwHNRJhhmFcbXvVDTqWMEVFBX6",             // SOL (~33 char)
"9WzDXwBbmkg8ZTbNMqUxvQRAyrZzDsGYdLVL9zYtAWWM",  // SOL (44 char)
"So11111111111111111111111111111111111111112",  // SOL (wrapped SOL mint)
// BTC P2PKH
  "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",   // ✅ valido (genesis)
  "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNb",   // ❌ checksum errato (a→b)

  // BTC P2SH
  "3J98t1WpEZ73CNmQviecrnyiWrnqRhWNLy",   // ✅ valido
  "3J98t1WpEZ73CNmQviecrnyiWrnqRhWNLz",   // ❌ checksum errato (y→z)

  // DOGE
  "DH5yaieqoZN36fDVciNyRueRGvGLR3mr7L",   // ✅ valido
  "DH5yaieqoZN36fDVciNyRueRGvGLR3mr7K",   // ❌ checksum errato (L→K)

  // Tron
  "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t",   // ✅ valido
  "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6u",   // ❌ checksum errato (t→u)
  // ✅ VALIDO (versione corretta di quello che ti avevo dato)
"0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed",
"0xfB6916095ca1df60bB79Ce92cE3Ea74c37c5d359",
"0xdbF03B407c01E7cD3CBea99509d93f8DDDC8C6FB",
"0xD1220A0cf47c7B9Be7A2E6BA89F429762e7b9aDb",
  }

  for _, w := range addresses{
	target := src.Wallet{
		Address: w,
	}
	err := target.Indentification()
	if(err!=nil){
		fmt.Println(err)
		continue
	}
	fmt.Printf("Wallet: %s\n",target.Address)
	fmt.Println(target.Type)
	fmt.Println(target.Subtype)
  }
}
