package handler

import (
	"encoding/json"
	"testing"
)

func TestMessage_BuildsTypedEnvelope(t *testing.T) {
	raw := Message(MsgQuestion, QuestionPayload{ID: "Q1", Text: "x", Options: []string{"a", "b"}, Order: 1, TimeLimitSeconds: 30})

	var env Envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	if env.Type != MsgQuestion {
		t.Errorf("type = %q, want %q", env.Type, MsgQuestion)
	}
	var p QuestionPayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if p.ID != "Q1" || p.TimeLimitSeconds != 30 {
		t.Errorf("payload = %+v, want Q1/30s", p)
	}
}

func TestParseClientMessage_SubmitAnswer(t *testing.T) {
	raw := []byte(`{"type":"submit_answer","payload":{"questionId":"Q1","answer":"went"}}`)
	env, err := ParseClientMessage(raw)
	if err != nil {
		t.Fatalf("ParseClientMessage = %v", err)
	}
	if env.Type != MsgSubmitAnswer {
		t.Fatalf("type = %q, want submit_answer", env.Type)
	}
	sa, err := env.AsSubmitAnswer()
	if err != nil {
		t.Fatalf("AsSubmitAnswer = %v", err)
	}
	if sa.QuestionID != "Q1" || sa.Answer != "went" {
		t.Errorf("payload = %+v, want Q1/went", sa)
	}
}

func TestParseClientMessage_InvalidJSON(t *testing.T) {
	if _, err := ParseClientMessage([]byte("not json")); err == nil {
		t.Errorf("ParseClientMessage(invalid) = nil error, want error")
	}
}
