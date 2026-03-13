package chat

import (
	"strings"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	openai_responses "github.com/openai/openai-go/responses"

	"vibecraft/backend/internal/store"
)

func joinProviderInstructions(base string, summary *string) string {
	parts := make([]string, 0, 2)
	if trimmed := strings.TrimSpace(base); trimmed != "" {
		parts = append(parts, trimmed)
	}
	if trimmed := strings.TrimSpace(stringOrEmpty(summary)); trimmed != "" {
		parts = append(parts, "Session summary:\n"+trimmed)
	}
	return strings.TrimSpace(strings.Join(parts, "\n\n"))
}

func trimMessagesForContext(messages []store.ChatMessage, keepRecent int) []store.ChatMessage {
	messages = filterModelContextMessages(messages)
	if keepRecent <= 0 || len(messages) <= keepRecent {
		return messages
	}
	return messages[len(messages)-keepRecent:]
}

func buildOpenAIReconstructedInput(summary *string, messages []store.ChatMessage, keepRecent int) (openai_responses.ResponseInputParam, string, error) {
	messages = trimMessagesForContext(messages, keepRecent)
	items := make(openai_responses.ResponseInputParam, 0, len(messages))
	for _, msg := range messages {
		contents, err := buildOpenAIMessageContent(msg.ContentText, msg.Attachments)
		if err != nil {
			return nil, "", err
		}
		role := openai_responses.EasyInputMessageRoleUser
		if strings.EqualFold(strings.TrimSpace(msg.Role), "assistant") {
			role = openai_responses.EasyInputMessageRoleAssistant
		}
		items = append(items, openai_responses.ResponseInputItemParamOfMessage(contents, role))
	}
	return items, renderConversationDebug(summary, messages, keepRecent), nil
}

func buildAnthropicReconstructedMessages(summary *string, messages []store.ChatMessage, keepRecent int) ([]anthropic.MessageParam, string, error) {
	messages = trimMessagesForContext(messages, keepRecent)
	out := make([]anthropic.MessageParam, 0, len(messages))
	for _, msg := range messages {
		blocks, err := buildAnthropicMessageBlocks(msg.ContentText, msg.Attachments)
		if err != nil {
			return nil, "", err
		}
		if strings.EqualFold(strings.TrimSpace(msg.Role), "assistant") {
			out = append(out, anthropic.NewAssistantMessage(blocks...))
			continue
		}
		out = append(out, anthropic.NewUserMessage(blocks...))
	}
	return out, renderConversationDebug(summary, messages, keepRecent), nil
}

func applyCurrentModelInput(messages []store.ChatMessage, currentUser store.ChatMessage, modelInput string) []store.ChatMessage {
	if len(messages) == 0 || strings.TrimSpace(currentUser.ID) == "" || strings.TrimSpace(modelInput) == "" {
		return messages
	}
	out := make([]store.ChatMessage, len(messages))
	copy(out, messages)
	for i := range out {
		if out[i].ID == currentUser.ID {
			out[i].ContentText = strings.TrimSpace(modelInput)
			break
		}
	}
	return out
}
