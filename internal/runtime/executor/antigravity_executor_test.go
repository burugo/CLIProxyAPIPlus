package executor

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetStableConversationText(t *testing.T) {
	// 1. Basic uncoated user text
	payload1 := `{"request": {"contents": [{"role": "user", "parts": [{"text": "Hello world"}]}]}}`
	text1 := getStableConversationText([]byte(payload1))
	assert.Equal(t, "Hello world", text1)

	// 2. Wrapped text containing ADDITIONAL_METADATA and Step Id
	payload2 := `{"request": {"contents": [{"role": "user", "parts": [{"text": "Step Id: 0\n\n<USER_REQUEST>\nHello world\n</USER_REQUEST>\n<ADDITIONAL_METADATA>\nThe current local time is: 2026-03-12T00:11:07+08:00\n</ADDITIONAL_METADATA>"}]}]}}`
	text2 := getStableConversationText([]byte(payload2))
	assert.Equal(t, "Hello world", text2)

	// 3. Ensure both generate identical UUIDs for Session and Request despite dynamic metadata
	sessionID1 := generateStableSessionID([]byte(payload1))
	sessionID2 := generateStableSessionID([]byte(payload2))
	assert.Equal(t, sessionID1, sessionID2)
	assert.NotEmpty(t, sessionID1)

	reqID1 := generateStableRequestID([]byte(payload1))
	reqID2 := generateStableRequestID([]byte(payload2))

	uuid1 := strings.Split(reqID1, "/")[2]
	uuid2 := strings.Split(reqID2, "/")[2]
	assert.Equal(t, uuid1, uuid2)
}

func TestGenerateStableRequestID_SequenceNumber(t *testing.T) {
	// Single user message: seq number should be 1-5
	payload1 := `{"request": {"contents": [{"role": "user", "parts": [{"text": "Hello"}]}]}}`
	reqID1 := generateStableRequestID([]byte(payload1))

	extractSeq := func(reqID string) int {
		parts := strings.Split(reqID, "/")
		seqStr := parts[len(parts)-1]
		var seq int
		fmt.Sscanf(seqStr, "%d", &seq)
		return seq
	}

	seq1 := extractSeq(reqID1)
	assert.True(t, seq1 >= 1 && seq1 <= 5, "Expected seq1 to be between 1 and 5, got %d", seq1)

	// User -> Model(FuncCall) -> User(FuncResp) -> Model(text):
	// Interactions: 4 parts total, seq number should be 4-20
	payload2 := `{
		"request": {
			"contents": [
				{"role": "user", "parts": [{"text": "Hello"}]},
				{"role": "model", "parts": [{"functionCall": {}}, {"functionCall": {}}]},
				{"role": "user", "parts": [{"functionResponse": {}}]},
				{"role": "model", "parts": [{"text": "Done"}]}
			]
		}
	}`
	reqID2 := generateStableRequestID([]byte(payload2))
	seq2 := extractSeq(reqID2)
	assert.True(t, seq2 >= 4 && seq2 <= 20, "Expected seq2 to be between 4 and 20, got %d", seq2)
}
