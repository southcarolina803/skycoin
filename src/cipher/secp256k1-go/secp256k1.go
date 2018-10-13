/*
Package secp256k1 provides private and public key generation with the secp256k1 elliptic curve.
*/
// nolint: golint
package secp256k1

import (
	"bytes"
	"encoding/hex"
	"log"

	secp "github.com/skycoin/skycoin/src/cipher/secp256k1-go/secp256k1-go2"
)

//intenal, may fail
//may return nil
func pubkeyFromSeckey(seckey []byte) []byte {
	if len(seckey) != 32 {
		log.Panic("seckey length invalid")
		return nil
	}

	if secp.SeckeyIsValid(seckey) != 1 {
		log.Panic("always ensure seckey is valid")
	}

	var pubkey = secp.GeneratePublicKey(seckey) //always returns true
	if pubkey == nil {
		log.Panic("ERROR: impossible, secp.BaseMultiply always returns true")
		return nil
	}
	if len(pubkey) != 33 {
		log.Panic("ERROR: impossible, invalid pubkey length")
	}

	if ret := secp.PubkeyIsValid(pubkey); ret != 1 {
		log.Panicf("ERROR: pubkey invald, ret=%d", ret)
		return nil
	}

	if ret := VerifyPubkey(pubkey); ret != 1 {

		log.Printf("seckey= %s", hex.EncodeToString(seckey))
		log.Printf("pubkey= %s", hex.EncodeToString(pubkey))
		log.Panicf("ERROR: pubkey verification failed, for deterministic. ret=%d", ret)
		return nil
	}

	return pubkey
}

// GenerateKeyPair generates public and private key pairs
func GenerateKeyPair() ([]byte, []byte) {
	const seckeyLen = 32

new_seckey:
	var seckey = RandByte(seckeyLen)
	if secp.SeckeyIsValid(seckey) != 1 {
		goto new_seckey //regen
	}

	pubkey := pubkeyFromSeckey(seckey)
	if pubkey == nil {
		log.Panic("IMPOSSIBLE: pubkey invalid from valid seckey")
		goto new_seckey
	}
	if ret := secp.PubkeyIsValid(pubkey); ret != 1 {
		log.Panicf("ERROR: Pubkey invalid, ret=%d", ret)
		goto new_seckey
	}

	return pubkey, seckey
}

// PubkeyFromSeckey must succeed
// TODO; hash on fail
// TOO: must match, result of private key from deterministic gen?
// deterministic gen will always return a valid private key
func PubkeyFromSeckey(seckey []byte) []byte {
	if len(seckey) != 32 {
		log.Panic("PubkeyFromSeckey: invalid length")
	}

	pubkey := pubkeyFromSeckey(seckey)
	if pubkey == nil {
		log.Panic("ERRROR: impossible, pubkey generation failed")
		//goto new_seckey
		return nil
	}
	if ret := secp.PubkeyIsValid(pubkey); ret != 1 {
		log.Panicf("ERROR: Pubkey invalid, ret=%d", ret)
		//goto new_seckey
		return nil
	}

	return pubkey
}

// UncompressPubkey uncompresses pubkey
func UncompressPubkey(pubkey []byte) []byte {
	if VerifyPubkey(pubkey) != 1 {
		log.Panic("cannot uncompress invalid pubkey")
		return nil
	}

	var pubXY secp.XY
	ok := pubXY.ParsePubkey(pubkey)
	if !ok {
		log.Panic("ERROR: impossible, pubkey parse fail")
	}

	var pubkey2 = pubXY.BytesUncompressed() //uncompressed
	if pubkey2 == nil {
		log.Panic("ERROR: pubkey, uncompression fail")
		return nil
	}

	return pubkey2
}

// UncompressedPubkeyFromSeckey returns nil on error
//should only need pubkey, not private key
//deprecate for _UncompressedPubkey
func UncompressedPubkeyFromSeckey(seckey []byte) []byte {

	if len(seckey) != 32 {
		log.Panic("PubkeyFromSeckey: invalid length")
	}

	pubkey := PubkeyFromSeckey(seckey)
	if pubkey == nil {
		log.Panic("Generating seckey from pubkey, failed")
		return nil
	}

	if VerifyPubkey(pubkey) != 1 {
		log.Panic("ERROR: impossible, Pubkey generation succeeded but pubkey validation failed")
	}

	var uncompressedPubkey = UncompressPubkey(pubkey)
	if uncompressedPubkey == nil {
		log.Panic("decompression failed")
		return nil
	}

	return uncompressedPubkey
}

//generates deterministic keypair with weak SHA256 hash of seed
//internal use only
//be extremely careful with golang slice semantics
func generateDeterministicKeyPair(seed []byte) ([]byte, []byte) {
	if seed == nil {
		log.Panic()
	}
	if len(seed) != 32 {
		log.Panic()
	}

	const seckeyLen = 32
	var seckey = make([]byte, seckeyLen)

new_seckey:
	seed = SumSHA256(seed[0:32])
	copy(seckey[0:32], seed[0:32])

	if !bytes.Equal(seckey, seed) {
		log.Panic()
	}
	if secp.SeckeyIsValid(seckey) != 1 {
		log.Printf("generateDeterministicKeyPair, secp.SeckeyIsValid fail")
		goto new_seckey //regen
	}

	var pubkey = secp.GeneratePublicKey(seckey)

	if pubkey == nil {
		log.Panic("ERROR: impossible, secp.BaseMultiply always returns true")
		goto new_seckey
	}
	if len(pubkey) != 33 {
		log.Panic("ERROR: impossible, pubkey length wrong")
	}

	if ret := secp.PubkeyIsValid(pubkey); ret != 1 {
		log.Panicf("ERROR: pubkey invalid, ret=%d", ret)
	}

	if ret := VerifyPubkey(pubkey); ret != 1 {
		log.Printf("seckey= %s", hex.EncodeToString(seckey))
		log.Printf("pubkey= %s", hex.EncodeToString(pubkey))

		log.Panicf("ERROR: pubkey is invalid, for deterministic. ret=%d", ret)
		goto new_seckey
	}

	return pubkey, seckey
}

// Secp256k1Hash double SHA256, salted with ECDH operation in curve
func Secp256k1Hash(hash []byte) []byte {
	hash = SumSHA256(hash)
	_, seckey := generateDeterministicKeyPair(hash)            //seckey1 is usually sha256 of hash
	pubkey, _ := generateDeterministicKeyPair(SumSHA256(hash)) //SumSHA256(hash) equals seckey usually
	ecdh := ECDH(pubkey, seckey)                               //raise pubkey to power of seckey in curve
	return SumSHA256(append(hash, ecdh...))                    //append signature to sha256(seed) and hash
}

//GenerateDeterministicKeyPair generate a single secure key
func GenerateDeterministicKeyPair(seed []byte) ([]byte, []byte) {
	_, pubkey, seckey := DeterministicKeyPairIterator(seed)
	return pubkey, seckey
}

// DeterministicKeyPairIterator iteratores for deterministic keypair generation. Returns SHA256, PubKey, SecKey as bytes
// Feeds SHA256 back into function to generate sequence of seckeys
// If private key is disclosed, should not be able to compute future or past keys in sequence
func DeterministicKeyPairIterator(seedIn []byte) ([]byte, []byte, []byte) {
	seed1 := Secp256k1Hash(seedIn) // make it difficult to derive future seckeys from previous seckeys
	seed2 := SumSHA256(append(seedIn, seed1...))
	pubkey, seckey := generateDeterministicKeyPair(seed2) // this is our seckey
	return seed1, pubkey, seckey
}

// Sign sign hash
func Sign(msg []byte, seckey []byte) []byte {
	if len(seckey) != 32 {
		log.Panic("Sign, Invalid seckey length")
	}
	if secp.SeckeyIsValid(seckey) != 1 {
		log.Panic("Attempting to sign with invalid seckey")
	}
	if len(msg) == 0 {
		log.Panic("Sign, message nil")
	}
	var nonce = RandByte(32)
	var sig = make([]byte, 65)
	var recid int

	var cSig secp.Signature

	var seckey1 secp.Number
	var msg1 secp.Number
	var nonce1 secp.Number

	seckey1.SetBytes(seckey)
	msg1.SetBytes(msg)
	nonce1.SetBytes(nonce)

	ret := cSig.Sign(&seckey1, &msg1, &nonce1, &recid)

	if ret != 1 {
		log.Panic("Secp25k1-go, Sign, signature operation failed")
	}

	sigBytes := cSig.Bytes()
	for i := 0; i < 64; i++ {
		sig[i] = sigBytes[i]
	}
	if len(sigBytes) != 64 {
		log.Fatalf("Invalid signature byte count: %d", len(sigBytes))
	}
	sig[64] = byte(int(recid)) // nolint: unconvert

	if int(recid) > 4 { // nolint: unconvert
		log.Panic()
	}

	return sig
}

// SignDeterministic generates signature in repeatable way
func SignDeterministic(msg []byte, seckey []byte, nonceSeed []byte) []byte {
	nonceSeed2 := SumSHA256(nonceSeed) //deterministicly generate nonce

	var sig = make([]byte, 65)
	var recid int

	var cSig secp.Signature

	var seckey1 secp.Number
	var msg1 secp.Number
	var nonce1 secp.Number

	seckey1.SetBytes(seckey)
	msg1.SetBytes(msg)
	nonce1.SetBytes(nonceSeed2)

	ret := cSig.Sign(&seckey1, &msg1, &nonce1, &recid)
	if ret != 1 {
		log.Panic("Secp256k1-go, SignDeterministic, signature fail")
	}

	sigBytes := cSig.Bytes()
	for i := 0; i < 64; i++ {
		sig[i] = sigBytes[i]
	}

	sig[64] = byte(recid)

	if len(sigBytes) != 64 {
		log.Fatalf("Invalid signature byte count: %d", len(sigBytes))
	}

	if int(recid) > 4 { // nolint: unconvert
		log.Panic()
	}

	return sig

}

// VerifySeckey renames ChkSeckeyValidity
func VerifySeckey(seckey []byte) int {
	if len(seckey) != 32 {
		return -1
	}

	//does conversion internally if less than order of curve
	if secp.SeckeyIsValid(seckey) != 1 {
		return -2
	}

	//seckey is just 32 bit integer
	//assume all seckey are valid
	//no. must be less than order of curve
	//note: converts internally
	return 1
}

/*
* Validate a public key.
*  Returns: 1: valid public key
*           0: invalid public key
 */

// VerifyPubkey renames ChkPubkeyValidity
// returns 1 on success
func VerifyPubkey(pubkey []byte) int {
	if len(pubkey) != 33 {
		//log.Printf("Seck256k1, VerifyPubkey, pubkey length invalid")
		return -1
	}

	if secp.PubkeyIsValid(pubkey) != 1 {
		return -3 //tests parse and validity
	}

	var pubkey1 secp.XY
	ret := pubkey1.ParsePubkey(pubkey)

	if !ret {
		return -2 //invalid, parse fail
	}
	//fails for unknown reason
	//TODO: uncomment
	if !pubkey1.IsValid() {
		return -4 //invalid, validation fail
	}
	return 1 //valid
}

// VerifySignatureValidity renames ChkSignatureValidity
func VerifySignatureValidity(sig []byte) int {
	//64+1
	if len(sig) != 65 {
		log.Fatal("1")
		return 0
	}
	//malleability check:
	//highest bit of 32nd byte must be 1
	//0x7f us 126 or 0b01111111
	if (sig[32] >> 7) == 1 {
		log.Fatal("2")
		return 0
	}
	//recovery id check
	if sig[64] >= 4 {
		log.Fatal("3")
		return 0
	}
	return 1
}

// VerifySignature for compressed signatures, does not need pubkey
// Rename SignatureChk
func VerifySignature(msg []byte, sig []byte, pubkey1 []byte) int {
	if msg == nil || sig == nil || pubkey1 == nil {
		log.Panic("VerifySignature, ERROR: invalid input, nils")
	}
	if len(sig) != 65 {
		log.Panic("VerifySignature, invalid signature length")
	}
	if len(pubkey1) != 33 {
		log.Panic("VerifySignature, invalid pubkey length")
	}

	//malleability check:
	//to enforce malleability, highest bit of S must be 1
	//S starts at 32nd byte
	//0x80 is 0b10000000 or 128 and masks highest bit
	if (sig[32] >> 7) == 1 {
		return 0 //valid signature, but fails malleability
	}

	if sig[64] >= 4 {
		return 0 //recover byte invalid
	}

	pubkey2 := RecoverPubkey(msg, sig) //if pubkey recovered, signature valid

	if pubkey2 == nil {
		return 0
	}

	if len(pubkey2) != 33 {
		log.Panic("recovered pubkey length invalid")
	}

	if !bytes.Equal(pubkey1, pubkey2) {
		return 0 //pubkeys do not match
	}

	return 1 //valid signature
}

//SignatureErrorString returns error string for signature failure
func SignatureErrorString(msg []byte, sig []byte, pubkey1 []byte) string {

	if msg == nil || len(sig) != 65 || len(pubkey1) != 33 {
		log.Panic()
	}

	if (sig[32] >> 7) == 1 {
		return "signature fails malleability requirement"
	}

	if sig[64] >= 4 {
		return "signature recovery byte is invalid, must be 0 to 3"
	}

	pubkey2 := RecoverPubkey(msg, sig) //if pubkey recovered, signature valid
	if pubkey2 == nil {
		return "pubkey from signature failed"
	}

	if !bytes.Equal(pubkey1, pubkey2) {
		return "input pubkey and recovered pubkey do not match"
	}

	return "No Error!"
}

// RecoverPubkey recovers the public key from the signature
//recovery of pubkey means correct signature
func RecoverPubkey(msg []byte, sig []byte) []byte {
	if len(sig) != 65 {
		log.Panic()
	}

	var recid = int(sig[64])

	pubkey, ret := secp.RecoverPublicKey(
		sig[0:64],
		msg,
		recid)

	if ret != 1 {
		log.Printf("RecoverPubkey: code %d", ret)
		return nil
	}
	//var pubkey2 []byte = pubkey1.Bytes() //compressed

	if pubkey == nil {
		log.Panic("ERROR: impossible, pubkey nil and ret ==1")
	}
	if len(pubkey) != 33 {
		log.Panic("pubkey length wrong")
	}

	return pubkey
	//nonce1.SetBytes(nonce_seed)

}

// ECDH raise a pubkey to the power of a seckey
func ECDH(pub []byte, sec []byte) []byte {
	if len(sec) != 32 {
		log.Panic()
	}

	if len(pub) != 33 {
		log.Panic()
	}

	if VerifySeckey(sec) != 1 {
		log.Printf("Invalid Seckey")
	}

	if ret := VerifyPubkey(pub); ret != 1 {
		log.Printf("Invalid Pubkey, %d", ret)
		return nil
	}

	pubkeyOut := secp.Multiply(pub, sec)
	if pubkeyOut == nil {
		return nil
	}
	if len(pubkeyOut) != 33 {
		log.Panic("ERROR: impossible, invalid pubkey length")
	}
	return pubkeyOut
}
