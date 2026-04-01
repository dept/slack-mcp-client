package slackbot

import (
	"testing"
	"time"

	"github.com/slack-go/slack/slackevents"
	"github.com/tuannvm/slack-mcp-client/internal/common/logging"
)

func TestShouldProcessEventRejectsDuplicateEventID(t *testing.T) {
	client := &Client{
		logger:       logging.New("test", logging.LevelError),
		recentEvents: make(map[string]time.Time),
	}

	event := slackevents.EventsAPIEvent{
		Type: slackevents.CallbackEvent,
		Data: &slackevents.EventsAPICallbackEvent{EventID: "Ev123"},
	}

	if !client.shouldProcessEvent(event) {
		t.Fatal("expected first event to be processed")
	}
	if client.shouldProcessEvent(event) {
		t.Fatal("expected duplicate event to be rejected")
	}
}

func TestShouldProcessEventRejectsDuplicateMessageFallbackKey(t *testing.T) {
	client := &Client{
		logger:       logging.New("test", logging.LevelError),
		recentEvents: make(map[string]time.Time),
	}

	message := &slackevents.MessageEvent{
		Channel:         "D123",
		User:            "U123",
		TimeStamp:       "1710000000.000100",
		ThreadTimeStamp: "1710000000.000100",
		Text:            "try again",
	}
	duplicate := slackevents.EventsAPIEvent{
		Type:       slackevents.CallbackEvent,
		InnerEvent: slackevents.EventsAPIInnerEvent{Data: message},
	}

	if !client.shouldProcessEvent(duplicate) {
		t.Fatal("expected first fallback-key event to be processed")
	}
	if client.shouldProcessEvent(duplicate) {
		t.Fatal("expected duplicate fallback-key event to be rejected")
	}
}

func TestShouldProcessEventAllowsEventAfterTTLExpiry(t *testing.T) {
	client := &Client{
		logger:       logging.New("test", logging.LevelError),
		recentEvents: make(map[string]time.Time),
	}

	key := "event:EvExpired"
	client.recentEvents[key] = time.Now().Add(-duplicateEventTTL - time.Second)

	event := slackevents.EventsAPIEvent{
		Type: slackevents.CallbackEvent,
		Data: &slackevents.EventsAPICallbackEvent{EventID: "EvExpired"},
	}
	if !client.shouldProcessEvent(event) {
		t.Fatal("expected expired duplicate entry to be treated as new")
	}
}
