package cryptstate

import (
	"bytes"
	"fmt"
	"testing"
)

func pair() (client, server *CryptState) {
	key := [16]byte{3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3}
	clientNonce := [16]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}
	serverNonce := [16]byte{2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2}
	return New(key, clientNonce, serverNonce), New(key, serverNonce, clientNonce)
}

func TestPacketsRoundTripInOrder(t *testing.T) {
	client, server := pair()
	for i := 0; i < 600; i++ {
		msg := []byte(fmt.Sprintf("voice packet %d", i))
		packet := client.Encrypt(msg)
		if len(packet) != len(msg)+4 {
			t.Fatalf("length %d", len(packet))
		}
		if got, ok := server.Decrypt(packet); !ok || !bytes.Equal(got, msg) {
			t.Fatalf("packet %d", i)
		}
	}
}

func TestLostAndLatePacketsStillDecrypt(t *testing.T) {
	client, server := pair()
	// Not all zeros: OCB2's countermeasure changes such blocks on purpose.
	data := func(i int) []byte { return bytes.Repeat([]byte{byte(0x41 + i)}, 20) }
	var packets [][]byte
	for i := 0; i < 10; i++ {
		packets = append(packets, client.Encrypt(data(i)))
	}
	for _, i := range []int{0, 3, 2, 4} { // 1 is lost, 2 is late
		if got, ok := server.Decrypt(packets[i]); !ok || !bytes.Equal(got, data(i)) {
			t.Fatalf("packet %d", i)
		}
	}
	if server.Good != 4 || server.Late != 1 {
		t.Fatalf("good %d late %d", server.Good, server.Late)
	}
}

func TestReplayedAndTamperedPacketsAreRefused(t *testing.T) {
	client, server := pair()
	first, second := client.Encrypt([]byte("hello")), client.Encrypt([]byte("world"))
	server.Decrypt(first)
	server.Decrypt(second)
	if _, ok := server.Decrypt(first); ok {
		t.Fatal("replay accepted")
	}
	bad := client.Encrypt([]byte("tampered"))
	bad[len(bad)-1] ^= 1
	if _, ok := server.Decrypt(bad); ok {
		t.Fatal("tampered packet accepted")
	}
	if _, ok := server.Decrypt([]byte{1, 2}); ok {
		t.Fatal("short packet accepted")
	}
}

func TestAWrongKeyDoesNotDecrypt(t *testing.T) {
	client, _ := pair()
	other := New([16]byte{4}, [16]byte{2}, [16]byte{1})
	if _, ok := other.Decrypt(client.Encrypt([]byte("secret"))); ok {
		t.Fatal("decrypted with the wrong key")
	}
}
