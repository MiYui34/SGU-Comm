// Package cryptstate is Mumble's UDP voice encryption: OCB2-AES128
// (CryptStateOCB2.cpp) with Mumble's countermeasure against the XEX* attack
// (eprint 2019/311, sec. 9), and the per-direction nonce handling.
package cryptstate

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
)

const blockSize = 16

// A 128-bit block as a big-endian number.
type block struct{ hi, lo uint64 }

func load(b []byte) block {
	return block{binary.BigEndian.Uint64(b), binary.BigEndian.Uint64(b[8:])}
}

func (x block) bytes() [16]byte {
	var b [16]byte
	binary.BigEndian.PutUint64(b[:], x.hi)
	binary.BigEndian.PutUint64(b[8:], x.lo)
	return b
}

func (x block) xor(y block) block { return block{x.hi ^ y.hi, x.lo ^ y.lo} }

// double is multiplication by x in GF(2^128) with the polynomial 0x87.
func (x block) double() block {
	out := block{x.hi<<1 | x.lo>>63, x.lo << 1}
	if x.hi>>63 == 1 {
		out.lo ^= 0x87
	}
	return out
}

func (x block) triple() block { return x.xor(x.double()) }

func enc(c cipher.Block, x block) block {
	b := x.bytes()
	c.Encrypt(b[:], b[:])
	return load(b[:])
}

func dec(c cipher.Block, x block) block {
	b := x.bytes()
	c.Decrypt(b[:], b[:])
	return load(b[:])
}

// The first byte in memory is the highest byte of the big-endian number.
var firstByteBit = block{1 << 56, 0}

func newCipher(key [16]byte) cipher.Block {
	c, err := aes.NewCipher(key[:])
	if err != nil {
		panic(err) // a 16-byte key never fails
	}
	return c
}

func allZero(b []byte) bool {
	for _, x := range b {
		if x != 0 {
			return false
		}
	}
	return true
}

// ocbEncrypt returns the ciphertext (same length) and the full tag.
func ocbEncrypt(c cipher.Block, nonce [16]byte, plain []byte) ([]byte, [16]byte) {
	delta := enc(c, load(nonce[:]))
	var checksum block
	out := make([]byte, 0, len(plain))
	rest := plain
	for len(rest) > blockSize {
		b := load(rest[:blockSize])
		// Countermeasure: an attack needs the second to last block to be
		// all zeros except its last byte. Flip a bit instead, as Mumble does
		// (digital silence produces such blocks).
		flip := len(rest)-blockSize <= blockSize && allZero(rest[:blockSize-1])
		delta = delta.double()
		tmp := delta.xor(b)
		if flip {
			tmp = tmp.xor(firstByteBit)
		}
		crypted := delta.xor(enc(c, tmp)).bytes()
		out = append(out, crypted[:]...)
		checksum = checksum.xor(b)
		if flip {
			checksum = checksum.xor(firstByteBit)
		}
		rest = rest[blockSize:]
	}
	delta = delta.double()
	pad := enc(c, delta.xor(block{0, uint64(len(rest)) * 8})).bytes()
	last := pad
	copy(last[:], rest)
	checksum = checksum.xor(load(last[:]))
	crypted := load(pad[:]).xor(load(last[:])).bytes()
	out = append(out, crypted[:len(rest)]...)
	return out, enc(c, delta.triple().xor(checksum)).bytes()
}

// ocbDecrypt returns the plaintext and the full tag (the caller compares
// the tag). ok is false if the packet matches the XEX* attack pattern.
func ocbDecrypt(c cipher.Block, nonce [16]byte, encrypted []byte) (plain []byte, tag [16]byte, ok bool) {
	delta := enc(c, load(nonce[:]))
	var checksum block
	out := make([]byte, 0, len(encrypted))
	rest := encrypted
	for len(rest) > blockSize {
		delta = delta.double()
		p := delta.xor(dec(c, delta.xor(load(rest[:blockSize]))))
		pb := p.bytes()
		out = append(out, pb[:]...)
		checksum = checksum.xor(p)
		rest = rest[blockSize:]
	}
	delta = delta.double()
	pad := enc(c, delta.xor(block{0, uint64(len(rest)) * 8}))
	var last [16]byte
	copy(last[:], rest)
	tmp := load(last[:]).xor(pad).bytes()
	checksum = checksum.xor(load(tmp[:]))
	out = append(out, tmp[:len(rest)]...)
	// Countermeasure: in an attack the decrypted last block equals delta
	// except for the last byte.
	d := delta.bytes()
	if bytes.Equal(tmp[:blockSize-1], d[:blockSize-1]) {
		return nil, tag, false
	}
	return out, enc(c, delta.triple().xor(checksum)).bytes(), true
}
