package slackbot

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/socketmode"

	customErrors "github.com/tuannvm/slack-mcp-client/internal/common/errors"
	"github.com/tuannvm/slack-mcp-client/internal/common/logging"
	"github.com/tuannvm/slack-mcp-client/internal/slack/formatter"
)

type UserFrontend interface {
	Run() error
	Ack(req socketmode.Request, payload ...interface{})
	GetEventChannel() chan socketmode.Event
	RemoveBotMention(msg string) string
	IsValidUser(userID string) bool
	GetLogger() *logging.Logger
	SendMessage(channelID, threadTS, text string)
	SendIntermediateMessage(channelID, threadTS, text string)
	GetThreadReplies(channelID, threadTS string) ([]slack.Message, error)
	GetUserInfo(userID string) (*UserProfile, error)
}

func getLogLevel(stdLogger *logging.Logger) logging.LogLevel {
	// Determine log level from environment variable
	logLevel := logging.LevelInfo // Default to INFO
	if envLevel := os.Getenv("LOG_LEVEL"); envLevel != "" {
		logLevel = logging.ParseLevel(envLevel)
		stdLogger.InfoKV("Setting Slack client log level from environment", "level", envLevel)
	}
	return logLevel
}

func GetSlackClient(botToken, appToken string, stdLogger *logging.Logger, thinkingMessage string) (*SlackClient, error) {
	if botToken == "" {
		return nil, fmt.Errorf("SLACK_BOT_TOKEN must be set")
	}
	if appToken == "" {
		return nil, fmt.Errorf("SLACK_APP_TOKEN must be set")
	}
	if !strings.HasPrefix(appToken, "xapp-") {
		return nil, fmt.Errorf("SLACK_APP_TOKEN must have the prefix \"xapp-\"")
	}

	logLevel := getLogLevel(stdLogger)

	// Create a structured logger for the Slack client
	slackLogger := logging.New("slack-client", logLevel)

	// Initialize the API client
	api := slack.New(
		botToken,
		slack.OptionAppLevelToken(appToken),
		// Still using standard logger for Slack API as it expects a standard logger
		slack.OptionLog(slackLogger.StdLogger()),
	)

	// Authenticate with Slack
	authTest, err := api.AuthTestContext(context.Background())
	if err != nil {
		return nil, customErrors.WrapSlackError(err, "authentication_failed", "Failed to authenticate with Slack")
	}

	mentionRegex := regexp.MustCompile(fmt.Sprintf("<@%s>", authTest.UserID))

	// Create the socket mode client
	client := socketmode.New(
		api,
		// Still using standard logger for socket mode as it expects a standard logger
		socketmode.OptionLog(slackLogger.StdLogger()),
		socketmode.OptionDebug(false),
	)

	return &SlackClient{
		Client:          client,
		botMentionRgx:   mentionRegex,
		botUserID:       authTest.UserID,
		logger:          slackLogger,
		thinkingMessage: ":loading1: " + thinkingMessage,
		userCache:       make(map[string]*UserProfile),
	}, nil
}

type UserProfile struct {
	userId   string
	realName string
	email    string
}

type SlackClient struct {
	*socketmode.Client
	botMentionRgx   *regexp.Regexp
	botUserID       string
	logger          *logging.Logger
	thinkingMessage string
	userCache       map[string]*UserProfile
	userCacheMu     sync.RWMutex // protects userCache
	thinkingMsgTS   sync.Map     // key: channelID:threadTS -> thinking message timestamp
}

func (slackClient *SlackClient) GetEventChannel() chan socketmode.Event {
	return slackClient.Events
}

func (slackClient *SlackClient) RemoveBotMention(msg string) string {
	return slackClient.botMentionRgx.ReplaceAllString(msg, "")
}

func (slackClient *SlackClient) GetLogger() *logging.Logger {
	return slackClient.logger
}

func (slackClient *SlackClient) IsValidUser(userID string) bool {
	return userID != "" && !slackClient.IsBotUser(userID)
}

func (slackClient *SlackClient) IsBotUser(userID string) bool {
	return userID == slackClient.botUserID
}

func (slackClient *SlackClient) GetThreadReplies(channelID, threadTS string) ([]slack.Message, error) {
	if channelID == "" || threadTS == "" {
		return nil, fmt.Errorf("channelID and threadTS must be provided")
	}
	replies, _, _, err := slackClient.GetConversationReplies(&slack.GetConversationRepliesParameters{
		ChannelID: channelID,
		Timestamp: threadTS,
	})
	if err != nil {
		return nil, customErrors.WrapSlackError(err, "fetch_thread_replies_failed", "Failed to fetch thread replies")
	}
	return replies, nil
}

func (slackClient *SlackClient) GetUserInfo(userID string) (*UserProfile, error) {
	if userID == "" {
		return nil, fmt.Errorf("userID must be provided")
	}
	slackClient.userCacheMu.RLock()
	if profile, ok := slackClient.userCache[userID]; ok {
		slackClient.userCacheMu.RUnlock()
		return profile, nil
	}
	slackClient.userCacheMu.RUnlock()

	slackProfile, err := slackClient.GetUserProfile(&slack.GetUserProfileParameters{
		UserID: userID,
	})
	if err != nil {
		return nil, customErrors.WrapSlackError(err, "fetch_user_profile_failed", "Failed to fetch user profile")
	}
	profile := &UserProfile{
		userId:   userID,
		realName: slackProfile.RealName,
		email:    slackProfile.Email,
	}
	slackClient.userCacheMu.Lock()
	slackClient.userCache[userID] = profile
	slackClient.userCacheMu.Unlock()
	return profile, nil
}

// SendIntermediateMessage sends an intermediate agent message to Slack without removing the thinking indicator.
func (slackClient *SlackClient) SendIntermediateMessage(channelID, threadTS, text string) {
	slackClient.sendMessageInternal(channelID, threadTS, text, false)
}

// SendMessage sends a message back to Slack, replying in a thread if threadTS is provided.
// It removes the thinking indicator before sending.
func (slackClient *SlackClient) SendMessage(channelID, threadTS, text string) {
	slackClient.sendMessageInternal(channelID, threadTS, text, true)
}

// sendMessageInternal is the shared implementation. When removeThinking is true, the
// thinking indicator is deleted before posting the message.
func (slackClient *SlackClient) sendMessageInternal(channelID, threadTS, text string, removeThinking bool) {
	if text == "" {
		slackClient.logger.WarnKV("Attempted to send empty message, skipping", "channel", channelID)
		return
	}

	threadKey := channelID + ":" + threadTS

	// Efficiently delete the thinking indicator using its stored timestamp (no API scan needed)
	if removeThinking {
		if ts, loaded := slackClient.thinkingMsgTS.LoadAndDelete(threadKey); loaded {
			_, _, err := slackClient.DeleteMessage(channelID, ts.(string))
			if err != nil {
				slackClient.logger.ErrorKV("Error deleting typing indicator message", "error", err)
			}
		}
	}

	// Detect message type and format accordingly
	messageType := formatter.DetectMessageType(text)
	slackClient.logger.DebugKV("Detected message type", "type", messageType, "length", len(text))

	var msgOptions []slack.MsgOption

	switch messageType {
	case formatter.JSONBlock:
		// Message is already in Block Kit JSON format
		options := formatter.DefaultOptions()
		options.Format = formatter.BlockFormat
		options.ThreadTS = threadTS
		msgOptions = formatter.FormatMessage(text, options)

	case formatter.StructuredData:
		// Convert structured data to Block Kit format
		formattedText := formatter.FormatStructuredData(text)
		options := formatter.DefaultOptions()
		options.Format = formatter.BlockFormat
		options.ThreadTS = threadTS
		msgOptions = formatter.FormatMessage(formattedText, options)

	case formatter.MarkdownText, formatter.PlainText:
		// Apply Markdown formatting and use default text formatting
		formattedText := formatter.FormatMarkdown(text)
		options := formatter.DefaultOptions()
		options.ThreadTS = threadTS
		msgOptions = formatter.FormatMessage(formattedText, options)
	}

	// Send the message
	_, msgTS, err := slackClient.PostMessage(channelID, msgOptions...)
	if err != nil {
		slackClient.logger.ErrorKV("Error posting message to channel", "channel", channelID, "error", err, "messageType", messageType)

		// If we get an error with Block Kit format, try falling back to plain text
		if messageType == formatter.JSONBlock || messageType == formatter.StructuredData {
			slackClient.logger.InfoKV("Falling back to plain text format due to Block Kit error", "channel", channelID)

			// Apply markdown formatting to the original text and send as plain text
			formattedText := formatter.FormatMarkdown(text)
			fallbackOptions := []slack.MsgOption{
				slack.MsgOptionText(formattedText, false),
			}
			if threadTS != "" {
				fallbackOptions = append(fallbackOptions, slack.MsgOptionTS(threadTS))
			}

			// Try sending with plain text format
			_, msgTS, fallbackErr := slackClient.PostMessage(channelID, fallbackOptions...)
			if fallbackErr != nil {
				slackClient.logger.ErrorKV("Error posting fallback message to channel", "channel", channelID, "error", fallbackErr)
			} else if text == slackClient.thinkingMessage {
				slackClient.thinkingMsgTS.Store(threadKey, msgTS)
			}
		}
		return
	}

	// Track the thinking indicator's timestamp for efficient deletion on next send
	if text == slackClient.thinkingMessage {
		slackClient.thinkingMsgTS.Store(threadKey, msgTS)
	}
}
