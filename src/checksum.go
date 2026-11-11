package src

import (
	"strings"
	"crypto/sha256"
	"encoding/hex"
	"math/big"
	"bytes"

	"golang.org/x/crypto/sha3"
)

// BECH32 / BECH32M (BIP173 / BIP350)

const (
	bech32Const  = 1
	bech32mConst = 0x2bc830a3
)

func bech32Polymod(values []int) int {
	gen := []int{0x3b6a57b2, 0x26508e6d, 0x1ea119fa, 0x3d4233dd, 0x2a1462b3}
	chk := 1

	for _, v := range values {
		b := chk >> 25
		chk = (chk&0x1ffffff)<<5 ^ v
		for i := 0; i < 5; i++ {
			if (b>>i)&1 == 1 {
				chk ^= gen[i]
			}
		}
	}
	return chk
}

func hrpExpand(hrp string) []int {
	var ret []int
	for _, c := range hrp {
		ret = append(ret, int(c)>>5) // high bits
	}
	ret = append(ret, 0)
	for _, c := range hrp {
		ret = append(ret, int(c)&31) // low bits
	}
	return ret
}

// 5-bit groups back to bytes; leftover bits must be zero padding
func convertBits5to8(data []int) ([]byte, bool) {
	acc, bits := 0, 0
	var out []byte
	for _, v := range data {
		acc = acc<<5 | v
		bits += 5
		for bits >= 8 {
			bits -= 8
			out = append(out, byte(acc>>bits))
		}
	}
	if bits >= 5 || (acc<<(8-bits))&0xff != 0 {
		return nil, false
	}
	return out, true
}

// Full segwit decode, not just the checksum: BIP350 ties the checksum to the witness
// version (v0 must be bech32, v1+ must be bech32m), and the program length is fixed.
func segwitDecode(addr string) (hrp string, version int, program []byte, ok bool) {
	// mixed case is explicitly invalid
	if addr != strings.ToLower(addr) && addr != strings.ToUpper(addr) {
		return "", 0, nil, false
	}
	addr = strings.ToLower(addr)
	if len(addr) > 90 {
		return "", 0, nil, false
	}

	sep := strings.LastIndexByte(addr, '1')
	if sep < 1 || sep+7 > len(addr) {
		return "", 0, nil, false
	}
	hrp = addr[:sep]

	var values []int
	for _, c := range addr[sep+1:] {
		v := strings.IndexRune(alphabetBech32, c)
		if v < 0 {
			return "", 0, nil, false
		}
		values = append(values, v)
	}

	constant := bech32Polymod(append(hrpExpand(hrp), values...))
	if constant != bech32Const && constant != bech32mConst {
		return "", 0, nil, false
	}

	data := values[:len(values)-6] // drop the checksum
	if len(data) < 1 {
		return "", 0, nil, false
	}
	version = data[0]
	if version > 16 {
		return "", 0, nil, false
	}
	if (version == 0) != (constant == bech32Const) {
		return "", 0, nil, false
	}

	program, ok = convertBits5to8(data[1:])
	if !ok || len(program) < 2 || len(program) > 40 {
		return "", 0, nil, false
	}
	if version == 0 && len(program) != 20 && len(program) != 32 {
		return "", 0, nil, false
	}
	return hrp, version, program, true
}

// EIP-55: uppercase a hex letter when its Keccak-256 nibble is >= 8
func checkSumEvm(wallet string)(bool){
	alphabet := "asdfghjklqwertyuiopzxcvbnm"
	var result string

	hash := sha3.NewLegacyKeccak256()
	lowerWallet := strings.ToLower(wallet[2:])

	hash.Write([]byte(lowerWallet))
	keccakHashRaw := hash.Sum(nil)
	keccakHash := hex.EncodeToString(keccakHashRaw)


	for i, c := range lowerWallet{

		if(strings.Contains(alphabet,string(c))){
			charHash := keccakHash[i]
			if(strings.Contains("89abcdef",string(charHash))){
				result += strings.ToUpper(string(c))
			}else{
				result += string(c)
			}
		}else{
			result += string(c)
		}

	}


	if(result == wallet[2:]){
		return true
	}

	return false
}


// Base58 to bytes. Leading zeros are lost by big.Int, so they are counted and restored:
// each leading alphabet[0] stands for one 0x00 byte.
func decodeBase58(s string, alphabet string) ([]byte, bool) {
	var num big.Int
	base := big.NewInt(58)

	for _, c := range s {
		index := strings.IndexRune(alphabet, c)
		if index < 0 {
			return nil, false
		}
		num.Mul(&num, base)
		num.Add(&num, big.NewInt(int64(index)))
	}

	zeros := 0
	for _, c := range s {
		if c != rune(alphabet[0]) {
			break
		}
		zeros++
	}

	raw := make([]byte, zeros)
	return append(raw, num.Bytes()...), true
}

// Base58Check (BTC / DOGE / TRON / XRP): last 4 bytes are sha256(sha256(payload)).
// Returns the payload, version byte included, so the caller can tell chains apart.
func decodeBase58Check(s string, alphabet string) ([]byte, bool) {
	raw, ok := decodeBase58(s, alphabet)
	if !ok || len(raw) < 5 {
		return nil, false
	}

	checksum := raw[len(raw)-4:]
	data := raw[:len(raw)-4]

	hash1 := sha256.Sum256(data)
	hash2 := sha256.Sum256(hash1[:])

	if !bytes.Equal(checksum, hash2[:4]) {
		return nil, false
	}
	return data, true
}

func checkSum(wallet string, alphabet string)(bool){
	_, ok := decodeBase58Check(wallet, alphabet)
	return ok
}

// Inverse of checkSum: appends sha256(sha256(payload))[:4] and encodes in Base58.
// Tron APIs return addresses as hex (0x41 + 20 bytes), this turns them into T...
func encodeBase58Check(payload []byte, alphabet string)(string){
	h1 := sha256.Sum256(payload)
	h2 := sha256.Sum256(h1[:])
	full := append(append([]byte{}, payload...), h2[:4]...)

	num := new(big.Int).SetBytes(full)
	base := big.NewInt(58)
	mod := new(big.Int)

	var out []byte
	for num.Sign() > 0 {
		num.DivMod(num, base, mod)
		out = append(out, alphabet[mod.Int64()])
	}
	// each leading 0x00 byte becomes the alphabet's first char
	for _, b := range full {
		if b != 0 {
			break
		}
		out = append(out, alphabet[0])
	}

	// digits came out least significant first
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return string(out)
}
