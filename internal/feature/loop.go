package feature

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
)

// LoopHeader carries stateless loop detection between Zelports:
//
//	X-Zelport-Loop: <nonce>.<mark>.<mark>…
//
// The first Zelport on a path picks a random 16-byte nonce for this request only. Every
// Zelport that serves a route adds an 8-byte mark, SHA-256(nonce, route ID, hostname, port).
// A request that brings back a mark the current route would add has passed this route before.
// Nothing is stored: route IDs are random UUIDs already in the configuration, and the nonce
// makes marks unlinkable between requests, so the header names no device, group or route.
const LoopHeader = "X-Zelport-Loop"

// LoopHopLimit bounds the marks a request may carry, and so the header size.
const LoopHopLimit = 8

const (
	loopNonceSize = 16
	loopMarkSize  = 8
)

var (
	ErrLoop         = errors.New("request passed this route before")
	ErrLoopHops     = errors.New("request passed too many Zelport routes")
	ErrLoopMetadata = errors.New("invalid loop metadata")
)

// AdvanceLoop checks a request's loop header for this route and returns the header to send on.
func AdvanceLoop(value, routeID, hostname string, port int) (string, error) {
	encoding := base64.RawURLEncoding.Strict()
	var nonce []byte
	var marks [][]byte
	if value == "" {
		nonce = make([]byte, loopNonceSize)
		if _, err := rand.Read(nonce); err != nil {
			return "", err
		}
	} else {
		parts := strings.Split(value, ".")
		if len(parts) > LoopHopLimit+1 {
			return "", ErrLoopHops
		}
		var err error
		if nonce, err = encoding.DecodeString(parts[0]); err != nil || len(nonce) != loopNonceSize {
			return "", ErrLoopMetadata
		}
		for _, part := range parts[1:] {
			mark, err := encoding.DecodeString(part)
			if err != nil || len(mark) != loopMarkSize {
				return "", ErrLoopMetadata
			}
			marks = append(marks, mark)
		}
	}
	mark := loopMark(nonce, routeID, hostname, port)
	for _, seen := range marks {
		if bytes.Equal(seen, mark) {
			return "", ErrLoop
		}
	}
	if len(marks) >= LoopHopLimit {
		return "", ErrLoopHops
	}
	out := make([]string, 0, len(marks)+2)
	out = append(out, encoding.EncodeToString(nonce))
	for _, seen := range marks {
		out = append(out, encoding.EncodeToString(seen))
	}
	out = append(out, encoding.EncodeToString(mark))
	return strings.Join(out, "."), nil
}

func loopMark(nonce []byte, routeID, hostname string, port int) []byte {
	h := sha256.New()
	h.Write([]byte("zelport-loop-v1\x00"))
	h.Write(nonce)
	for _, part := range []string{routeID, strings.ToLower(hostname), strconv.Itoa(port)} {
		h.Write([]byte{0})
		h.Write([]byte(part))
	}
	return h.Sum(nil)[:loopMarkSize]
}
